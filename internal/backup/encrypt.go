package backup

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// Encryptor 备份加密器
type Encryptor struct {
	password string
	key      []byte
}

// NewEncryptor 创建加密器
func NewEncryptor(password string) (*Encryptor, error) {
	if password == "" {
		return nil, fmt.Errorf("密码不能为空")
	}

	// 使用 SHA256 生成 32 字节密钥
	hasher := sha256.New()
	hasher.Write([]byte(password))
	key := hasher.Sum(nil)

	return &Encryptor{
		password: password,
		key:      key,
	}, nil
}

// EncryptFile 使用 OpenSSL 加密文件
func (e *Encryptor) EncryptFile(inputPath, outputPath string) error {
	// 使用 OpenSSL AES-256-CBC 加密
	cmd := exec.Command(
		"openssl",
		"enc",
		"-aes-256-cbc",
		"-salt",
		"-pbkdf2",
		"-iter", "100000",
		"-in", inputPath,
		"-out", outputPath,
		"-pass", "pass:"+e.password,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("OpenSSL 加密失败：%w, output: %s", err, string(output))
	}

	return nil
}

// DecryptFile 使用 OpenSSL 解密文件
func (e *Encryptor) DecryptFile(inputPath, outputPath string) error {
	// 使用 OpenSSL AES-256-CBC 解密
	cmd := exec.Command(
		"openssl",
		"enc",
		"-aes-256-cbc",
		"-d",
		"-pbkdf2",
		"-iter", "100000",
		"-in", inputPath,
		"-out", outputPath,
		"-pass", "pass:"+e.password,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("OpenSSL 解密失败：%w, output: %s", err, string(output))
	}

	return nil
}

// EncryptDirectory 加密整个目录
func (e *Encryptor) EncryptDirectory(inputDir, outputDir string) error {
	// 先打包成 tar
	tarPath := outputDir + ".tar"
	if err := createTar(inputDir, tarPath); err != nil {
		return fmt.Errorf("创建 tar 失败：%w", err)
	}

	// 加密 tar 文件
	encryptedPath := tarPath + ".enc"
	if err := e.EncryptFile(tarPath, encryptedPath); err != nil {
		os.Remove(tarPath)
		return fmt.Errorf("加密失败：%w", err)
	}

	// 删除未加密的 tar
	os.Remove(tarPath)

	return nil
}

// DecryptDirectory 解密目录
func (e *Encryptor) DecryptDirectory(encryptedPath, outputDir string) error {
	// 解密文件
	decryptedTar := encryptedPath + ".dec.tar"
	if err := e.DecryptFile(encryptedPath, decryptedTar); err != nil {
		return fmt.Errorf("解密失败：%w", err)
	}

	// 解压 tar
	if err := extractTar(decryptedTar, outputDir); err != nil {
		os.Remove(decryptedTar)
		return fmt.Errorf("解压失败：%w", err)
	}

	// 删除临时 tar
	os.Remove(decryptedTar)

	return nil
}

// createTar 创建 tar 包
func createTar(srcDir, dstTar string) error {
	cmd := exec.Command("tar", "cf", dstTar, "-C", filepath.Dir(srcDir), filepath.Base(srcDir))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar 创建失败：%w, output: %s", err, string(output))
	}
	return nil
}

// extractTar 解压 tar 包
func extractTar(srcTar, dstDir string) error {
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	cmd := exec.Command("tar", "xf", srcTar, "-C", dstDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar 解压失败：%w, output: %s", err, string(output))
	}
	return nil
}

// ========== Go 原生加密实现（备用方案）==========

// EncryptData 使用 AES-GCM 加密数据（Go 原生实现）
func (e *Encryptor) EncryptData(data []byte) ([]byte, error) {
	// 创建 AES cipher
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("创建 cipher 失败：%w", err)
	}

	// 创建 GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败：%w", err)
	}

	// 生成 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成 nonce 失败：%w", err)
	}

	// 加密数据
	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	return ciphertext, nil
}

// DecryptData 使用 AES-GCM 解密数据（Go 原生实现）
func (e *Encryptor) DecryptData(ciphertext []byte) ([]byte, error) {
	// 创建 AES cipher
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("创建 cipher 失败：%w", err)
	}

	// 创建 GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败：%w", err)
	}

	// 验证 nonce 大小
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("密文太短")
	}

	// 提取 nonce 和密文
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// 解密数据
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败：%w", err)
	}

	return plaintext, nil
}

// EncryptStream 流式加密（适用于大文件）
func (e *Encryptor) EncryptStream(input io.Reader, output io.Writer) error {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	// 生成 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}

	// 写入 nonce
	if _, err := output.Write(nonce); err != nil {
		return err
	}

	// 读取全部数据并加密（简化实现，大文件需要分块）
	data, err := io.ReadAll(input)
	if err != nil {
		return err
	}

	ciphertext := gcm.Seal(nil, nonce, data, nil)
	_, err = output.Write(ciphertext)
	return err
}

// DecryptStream 流式解密
func (e *Encryptor) DecryptStream(input io.Reader, output io.Writer) error {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	// 读取 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(input, nonce); err != nil {
		return err
	}

	// 读取密文
	ciphertext, err := io.ReadAll(input)
	if err != nil {
		return err
	}

	// 解密
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return err
	}

	_, err = output.Write(plaintext)
	return err
}

// GenerateKey 生成随机密钥
func GenerateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", err
	}

	// 返回 base64 编码的密钥
	return fmt.Sprintf("%x", key), nil
}

// EncryptBackupWithGPG 使用 GPG 加密（如果系统安装了 GPG）
func EncryptBackupWithGPG(inputPath, outputPath, recipient string) error {
	cmd := exec.Command(
		"gpg",
		"--yes",
		"--batch",
		"--encrypt",
		"--recipient", recipient,
		"--output", outputPath,
		inputPath,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("GPG 加密失败：%w, output: %s", err, string(output))
	}

	return nil
}

// DecryptBackupWithGPG 使用 GPG 解密
func DecryptBackupWithGPG(inputPath, outputPath string) error {
	cmd := exec.Command(
		"gpg",
		"--yes",
		"--batch",
		"--decrypt",
		"--output", outputPath,
		inputPath,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("GPG 解密失败：%w, output: %s", err, string(output))
	}

	return nil
}

// VerifyIntegrity 验证备份完整性（使用 checksum）
func VerifyIntegrity(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// 计算 SHA256
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash), nil
}

// WriteChecksum 写入校验和文件
func WriteChecksum(filePath string) error {
	checksum, err := VerifyIntegrity(filePath)
	if err != nil {
		return err
	}

	checksumPath := filePath + ".sha256"
	return os.WriteFile(checksumPath, []byte(checksum+"  "+filepath.Base(filePath)), 0644)
}

// VerifyChecksum 验证校验和
func VerifyChecksum(filePath string) (bool, error) {
	checksumPath := filePath + ".sha256"
	
	expectedData, err := os.ReadFile(checksumPath)
	if err != nil {
		return false, err
	}

	expected := string(bytes.Fields(expectedData)[0])
	actual, err := VerifyIntegrity(filePath)
	if err != nil {
		return false, err
	}

	return expected == actual, nil
}
