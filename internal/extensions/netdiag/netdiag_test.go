package netdiag

import (
	"testing"
)

func TestNewDiagnoser(t *testing.T) {
	d := NewDiagnoser()
	if d == nil {
		t.Fatal("NewDiagnoser 返回 nil")
	}
	if d.thresholds == nil {
		t.Fatal("阈值不应为空")
	}
}

func TestRegisterInterface(t *testing.T) {
	d := NewDiagnoser()
	iface := &NetworkInterface{
		Name: "eth0", IP: "192.168.1.100", Speed: 1000, MTU: 1500, Status: "up",
	}
	d.RegisterInterface(iface)

	if len(d.interfaces) != 1 {
		t.Errorf("接口数应为 1, 实际 %d", len(d.interfaces))
	}

	// 更新已存在的接口
	iface2 := &NetworkInterface{
		Name: "eth0", IP: "192.168.1.200", Speed: 2500, MTU: 9000, Status: "up",
	}
	d.RegisterInterface(iface2)

	if len(d.interfaces) != 1 {
		t.Errorf("更新后接口数应仍为 1, 实际 %d", len(d.interfaces))
	}
	if d.interfaces[0].Speed != 2500 {
		t.Errorf("速度应更新为 2500, 实际 %d", d.interfaces[0].Speed)
	}
}

func TestFullDiagnosis(t *testing.T) {
	d := NewDiagnoser()
	d.RegisterInterface(&NetworkInterface{
		Name: "eth0", IP: "192.168.1.100", Speed: 1000, MTU: 1500, Status: "up",
	})

	report := d.RunFullDiagnosis()
	if report == nil {
		t.Fatal("报告不应为空")
	}
	if report.Score != 100 {
		t.Errorf("无问题时应得 100 分, 实际 %d", report.Score)
	}
	if report.Overall != SeverityOK {
		t.Errorf("无问题时应为 OK, 实际 %s", report.Overall)
	}
}

func TestMTUWarning(t *testing.T) {
	d := NewDiagnoser()
	d.RegisterInterface(&NetworkInterface{
		Name: "eth0", IP: "192.168.1.100", Speed: 1000, MTU: 1400, Status: "up",
	})

	report := d.RunFullDiagnosis()
	if report.Score >= 100 {
		t.Error("MTU 偏低应扣分")
	}
	found := false
	for _, issue := range report.Issues {
		if issue.Title == "MTU 偏低" {
			found = true
			break
		}
	}
	if !found {
		t.Error("应检测到 MTU 偏低警告")
	}
}

func TestNetworkErrors(t *testing.T) {
	d := NewDiagnoser()
	d.RegisterInterface(&NetworkInterface{
		Name: "eth0", IP: "192.168.1.100", Speed: 1000, MTU: 1500,
		Status: "up", RxErrors: 10, TxErrors: 5,
	})

	report := d.RunFullDiagnosis()
	if report.Score >= 100 {
		t.Error("网络错误应扣分")
	}
}

func TestRunDiagnosis(t *testing.T) {
	d := NewDiagnoser()
	diag := d.RunDiagnosis(DiagConnectivity, "192.168.1.1")
	if diag == nil {
		t.Fatal("诊断不应为空")
	}
	if diag.Type != DiagConnectivity {
		t.Errorf("类型应为 connectivity, 实际 %s", diag.Type)
	}
}

func TestGetHistory(t *testing.T) {
	d := NewDiagnoser()
	d.RunDiagnosis(DiagConnectivity, "192.168.1.1")
	d.RunDiagnosis(DiagDNS, "8.8.8.8")

	history := d.GetHistory(10)
	if len(history) != 2 {
		t.Errorf("历史应有 2 条, 实际 %d", len(history))
	}
}

func TestFormatReport(t *testing.T) {
	d := NewDiagnoser()
	d.RegisterInterface(&NetworkInterface{
		Name: "eth0", IP: "192.168.1.100", Speed: 2500, MTU: 9000, Status: "up",
	})
	report := d.RunFullDiagnosis()
	output := d.FormatReport(report)
	if output == "" {
		t.Error("格式化报告不应为空")
	}
}
