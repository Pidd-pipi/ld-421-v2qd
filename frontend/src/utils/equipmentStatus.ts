import { AssetStatus } from '../types'

export const ASSET_STATUS_TEXT: Record<string, string> = {
  [AssetStatus.Available]: '可借',
  [AssetStatus.InUse]: '使用中',
  [AssetStatus.Maintenance]: '维护中',
  [AssetStatus.Retired]: '已报废',
  [AssetStatus.Lost]: '已丢失'
}

// 设备是否可借用/预约：仅“可借”状态开放。
export function isEquipmentAvailable(status?: string): boolean {
  return status === AssetStatus.Available
}

// 设备不可用时的统一提示，设备列表、借用页与预约页共用同一文案。
export function unavailableReason(status?: string): string {
  switch (status) {
    case AssetStatus.Maintenance:
      return '设备维护中，暂不可用'
    case AssetStatus.Lost:
      return '设备已丢失，暂不可用'
    case AssetStatus.InUse:
      return '设备使用中，暂不可用'
    case AssetStatus.Retired:
      return '设备已报废，暂不可用'
    default:
      return '设备当前不可用'
  }
}

export function equipmentStatusText(status?: string): string {
  return (status && ASSET_STATUS_TEXT[status]) || status || '未知'
}
