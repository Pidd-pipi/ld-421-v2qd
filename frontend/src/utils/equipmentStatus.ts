import { AssetStatus } from '../types/enums'

// 设备状态展示信息：设备列表、借用页、预约页共用同一套状态文案与颜色。
export const ASSET_STATUS_META: Record<string, { color: string; text: string }> = {
  [AssetStatus.Available]: { color: 'green', text: '可借' },
  [AssetStatus.InUse]: { color: 'blue', text: '使用中' },
  [AssetStatus.Maintenance]: { color: 'orange', text: '维护中' },
  [AssetStatus.Retired]: { color: 'default', text: '已报废' },
  [AssetStatus.Lost]: { color: 'red', text: '已丢失' }
}

export const assetStatusText = (status: string): string => ASSET_STATUS_META[status]?.text ?? status

// 仅“可借”状态可以发起借用，与后端规则保持一致。
export const isBorrowable = (status: string): boolean => status === AssetStatus.Available

// 借用中/可借可预约未来时段；维护中、已丢失、已报废不可预约。
export const isReservable = (status: string): boolean =>
  status === AssetStatus.Available || status === AssetStatus.InUse

export const unavailableHint = (status: string): string => {
  switch (status) {
    case AssetStatus.Maintenance:
      return '该设备维护中，暂不可用'
    case AssetStatus.Lost:
      return '该设备已丢失，暂不可用'
    case AssetStatus.Retired:
      return '该设备已报废，暂不可用'
    case AssetStatus.InUse:
      return '该设备正在借用中，暂不可借用'
    default:
      return '该设备当前不可用'
  }
}
