package scanner

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"yx-daq/internal/types"
)

// DAQScanner XY-DAQ/DAQ-T UDP设备扫描器（DAQ8/DAQ16/DAQ-T-1603通用）
type DAQScanner struct {
	listenPort    int
	broadcastPort int
}

// daqTDiscoveryResponse DAQ-T设备JSON发现响应
type daqTDiscoveryResponse struct {
	IP              string `json:"ip"`
	MAC             string `json:"mac"`
	SerialNumber    string `json:"serialNumber"`
	Model           string `json:"model"`
	FirmwareVersion string `json:"firmwareVersion"`
	Port            int    `json:"port"`
}

// NewDAQScanner 创建扫描器
func NewDAQScanner() *DAQScanner {
	return &DAQScanner{
		listenPort:    7001,
		broadcastPort: 7000,
	}
}

// Scan 执行UDP广播扫描
func (s *DAQScanner) Scan(timeoutMs int) ([]types.DiscoveredDevice, error) {
	if timeoutMs == 0 {
		timeoutMs = 3000
	}

	// 创建UDP监听
	listener, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", s.listenPort))
	if err != nil {
		return nil, fmt.Errorf("create UDP listener failed: %w", err)
	}
	defer listener.Close()

	// 获取所有网卡广播地址
	broadcastAddrs, err := s.getBroadcastAddresses()
	if err != nil {
		return nil, err
	}

	// 发送广播（同时支持XY-DAQ和DAQ-T设备）
	broadcastMsgs := [][]byte{[]byte("psi9000"), []byte("T1603")}
	for _, addr := range broadcastAddrs {
		target := fmt.Sprintf("%s:%d", addr, s.broadcastPort)
		conn, err := net.Dial("udp4", target)
		if err != nil {
			continue
		}
		for _, msg := range broadcastMsgs {
			conn.Write(msg)
		}
		conn.Close()
	}

	// 监听响应
	devices := []types.DiscoveredDevice{}
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	buf := make([]byte, 1024)

	for time.Now().Before(deadline) {
		listener.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, _, err := listener.ReadFrom(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			break
		}

		if device, ok := s.parseResponse(string(buf[:n])); ok {
			devices = append(devices, device)
		}
	}

	return devices, nil
}

// parseResponse 解析扫描响应
// 支持两种格式：
//   - JSON（DAQ-T设备）: {"ip":"...","mac":"...","serialNumber":"...","model":"...","firmwareVersion":"...","port":9000,...}
//   - CSV（XY-DAQ设备）: IP,MAC,_,SN,FW,_,_,Port,Mask,GW
func (s *DAQScanner) parseResponse(resp string) (types.DiscoveredDevice, bool) {
	// 尝试JSON格式解析（DAQ-T设备）
	if strings.HasPrefix(strings.TrimSpace(resp), "{") {
		var jr daqTDiscoveryResponse
		if err := json.Unmarshal([]byte(resp), &jr); err == nil && jr.IP != "" {
			return types.DiscoveredDevice{
				IP:       jr.IP,
				MAC:      jr.MAC,
				SN:       jr.SerialNumber,
				Firmware: jr.FirmwareVersion,
				Port:     jr.Port,
			}, true
		}
	}

	// 回退到CSV格式解析（XY-DAQ设备）
	parts := strings.Split(resp, ",")
	if len(parts) < 10 {
		return types.DiscoveredDevice{}, false
	}

	device := types.DiscoveredDevice{
		IP:       strings.TrimSpace(parts[0]),
		MAC:      strings.TrimSpace(parts[1]),
		SN:       strings.TrimSpace(parts[3]),
		Firmware: strings.TrimSpace(parts[4]),
		Mask:     strings.TrimSpace(parts[8]),
		Gateway:  strings.TrimSpace(parts[9]),
	}

	// 解析端口
	fmt.Sscanf(strings.TrimSpace(parts[7]), "%d", &device.Port)

	return device, true
}

// getBroadcastAddresses 获取广播地址
func (s *DAQScanner) getBroadcastAddresses() ([]string, error) {
	addrs := []string{"255.255.255.255"}

	interfaces, err := net.Interfaces()
	if err != nil {
		return addrs, nil
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagBroadcast == 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		ifaceAddrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range ifaceAddrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
				broadcast := s.calcBroadcast(ipNet)
				if broadcast != "" {
					addrs = append(addrs, broadcast)
				}
			}
		}
	}

	return addrs, nil
}

// calcBroadcast 计算广播地址
func (s *DAQScanner) calcBroadcast(ipNet *net.IPNet) string {
	ip := ipNet.IP.To4()
	mask := ipNet.Mask
	if len(ip) != 4 || len(mask) != 4 {
		return ""
	}

	broadcast := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		broadcast[i] = ip[i] | ^mask[i]
	}

	return broadcast.String()
}
