package types

import "fmt"

// PressureUnitCoefficients 压力单位换算系数表（1 psi = coeff 该单位）
// 与驱动层 driver/xy_daq16.go 的 unitToCoeff 系数保持一致
var PressureUnitCoefficients = map[string]float64{
	"psi":     1,
	"kgf/cm²": 0.07031,
	"bar":     0.0689476,
	"mbar":    68.9476,
	"kPa":     6.89476,
	"MPa":     0.00689476,
	"Pa":      6894.76,
	"mmHg":    51.7149,
	"atm":     0.068046,
}

// psiToPaCoeff 1 psi 对应的 Pa 值
const psiToPaCoeff = 6894.76

// ZeroCalibrationSupportedUnits 零位校准支持的压力单位白名单
// 设备协议支持 9 种，但 UI 与校零白名单裁剪为 6 种（删除 mmHg/atm/mbar）
var ZeroCalibrationSupportedUnits = []string{"psi", "kgf/cm²", "bar", "kPa", "MPa", "Pa"}

// IsZeroCalibrationSupported 判断单位是否在零位校准白名单内
func IsZeroCalibrationSupported(unit string) bool {
	for _, u := range ZeroCalibrationSupportedUnits {
		if u == unit {
			return true
		}
	}
	return false
}

// ConvertPressureToPa 将任意压力单位值换算到 Pa
// coeff = 1 psi 对应的该单位值，故 1 该单位 = psiToPaCoeff / coeff Pa
func ConvertPressureToPa(value float64, fromUnit string) (float64, error) {
	coeff, ok := PressureUnitCoefficients[fromUnit]
	if !ok {
		return 0, fmt.Errorf("unsupported pressure unit: %s", fromUnit)
	}
	return value * psiToPaCoeff / coeff, nil
}

// ConvertPaToUnit 将 Pa 值换算到目标压力单位
func ConvertPaToUnit(value float64, toUnit string) (float64, error) {
	coeff, ok := PressureUnitCoefficients[toUnit]
	if !ok {
		return 0, fmt.Errorf("unsupported pressure unit: %s", toUnit)
	}
	return value * coeff / psiToPaCoeff, nil
}

// ConvertPressureToUnit 将任意压力单位值换算到目标压力单位
// 经 Pa 中转：from → Pa → to
func ConvertPressureToUnit(value float64, fromUnit, toUnit string) (float64, error) {
	if fromUnit == toUnit {
		return value, nil
	}
	inPa, err := ConvertPressureToPa(value, fromUnit)
	if err != nil {
		return 0, err
	}
	return ConvertPaToUnit(inPa, toUnit)
}
