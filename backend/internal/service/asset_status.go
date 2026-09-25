package service

import (
	"github.com/labequipment/lab-equipment/internal/constants"
)

// assetBorrowable 判断设备当前是否可借用：仅“可借”状态允许。
func assetBorrowable(status constants.AssetStatus) bool {
	return status == constants.AssetStatusAvailable
}

// assetReservable 判断设备当前是否可预约：维护中、丢失、报废均不可预约；
// 借用中的设备仍可预约未来时段。
func assetReservable(status constants.AssetStatus) bool {
	return status == constants.AssetStatusAvailable || status == constants.AssetStatusInUse
}

// unavailableReason 返回设备不可用状态对应的明确提示。
func unavailableReason(status constants.AssetStatus) string {
	switch status {
	case constants.AssetStatusMaintenance:
		return "设备维护中，暂不可用"
	case constants.AssetStatusLost:
		return "设备已丢失，暂不可用"
	case constants.AssetStatusRetired:
		return "设备已报废，暂不可用"
	case constants.AssetStatusInUse:
		return "设备正在借用中，暂不可用"
	default:
		return "设备当前不可用"
	}
}
