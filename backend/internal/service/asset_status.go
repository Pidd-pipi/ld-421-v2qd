package service

import "github.com/labequipment/lab-equipment/internal/constants"

// unavailableMessage 返回设备资产状态对应的不可借用/不可预约提示。
func unavailableMessage(status constants.AssetStatus) string {
	switch status {
	case constants.AssetStatusMaintenance:
		return "设备处于维护中，暂不可用"
	case constants.AssetStatusLost:
		return "设备已丢失，暂不可用"
	case constants.AssetStatusInUse:
		return "设备使用中，暂不可用"
	case constants.AssetStatusRetired:
		return "设备已报废，暂不可用"
	default:
		return "设备当前不可用"
	}
}

// reservationApproveBlocked 预约审批时，仅维护/丢失/报废明确阻断；
// 使用中的设备可能在预约时段开始前归还，不在此阻断。
func reservationApproveBlocked(status constants.AssetStatus) bool {
	switch status {
	case constants.AssetStatusMaintenance, constants.AssetStatusLost, constants.AssetStatusRetired:
		return true
	default:
		return false
	}
}
