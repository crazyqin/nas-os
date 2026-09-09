// listener.go - 同端口双协议（TLS + 明文回退）监听器.
package webtls

import (
	"crypto/tls"
	"io"
	"net"
	"time"
)

// peekTimeout 预读首字节的等待上限：正常客户端（浏览器/curl/探针）连接后
// 毫秒级即发出 ClientHello 或请求行；空连接（端口扫描）在此超时后被丢弃.
const peekTimeout = 5 * time.Second

// Listener 实现同端口双协议：先预读首字节探测 TLS ClientHello（0x16），
// 是 TLS 则交给 tls.Server 握手，否则按明文 HTTP 处理。这样：
//   - Docker/K8s HTTP 探针、nasctl、curl http:// 不会被 TLS 握手失败打断；
//   - 非本机明文请求由上层（web.httpsRedirectMiddleware）决定是否跳转 HTTPS.
type Listener struct {
	inner net.Listener
	cfg   *tls.Config
}

// NewListener 包装一个已监听的 net.Listener 为双协议监听器.
func NewListener(inner net.Listener, tlsCfg *tls.Config) *Listener {
	return &Listener{inner: inner, cfg: tlsCfg}
}

// Accept 等待并返回下一个连接（*tls.Conn 或原样明文连接）.
// 预读失败（超时/对端关闭）的空连接直接丢弃并继续等待，
// 不向 http.Serve 的 Accept 循环注入错误.
func (l *Listener) Accept() (net.Conn, error) {
	for {
		conn, err := l.inner.Accept()
		if err != nil {
			return nil, err
		}
		wrapped, err := l.wrap(conn)
		if err != nil {
			_ = conn.Close()
			continue
		}
		return wrapped, nil
	}
}

// wrap 预读首字节判定协议并包装连接.
func (l *Listener) wrap(conn net.Conn) (net.Conn, error) {
	first, err := peekFirstByte(conn)
	if err != nil {
		return nil, err
	}
	plain := &peekConn{Conn: conn, buffered: first}
	if first != 0x16 { // 非 TLS 记录层 → 明文
		return plain, nil
	}
	return tls.Server(plain, l.cfg), nil
}

// Close 关闭底层监听器.
func (l *Listener) Close() error { return l.inner.Close() }

// Addr 返回底层监听地址.
func (l *Listener) Addr() net.Addr { return l.inner.Addr() }

// peekFirstByte 带超时阻塞预读一个字节（探测后归还读流，不消耗数据）.
func peekFirstByte(conn net.Conn) (byte, error) {
	_ = conn.SetReadDeadline(time.Now().Add(peekTimeout))
	defer func() { _ = conn.SetReadDeadline(time.Time{}) }()
	var buf [1]byte
	if _, err := io.ReadFull(conn, buf[:]); err != nil {
		return 0, err
	}
	return buf[0], nil
}

// peekConn 把预读的字节塞回读流最前.
type peekConn struct {
	net.Conn
	buffered byte
	readOnce bool
}

func (p *peekConn) Read(b []byte) (int, error) {
	if !p.readOnce {
		if len(b) == 0 {
			return 0, nil
		}
		p.readOnce = true
		b[0] = p.buffered
		return 1, nil
	}
	return p.Conn.Read(b)
}
