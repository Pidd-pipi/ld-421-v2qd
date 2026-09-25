import { Select } from 'antd'
import type { Equipment } from '../../types'
import { equipmentStatusText, isEquipmentAvailable } from '../../utils/equipmentStatus'

interface EquipmentSelectProps {
  equipment: Equipment[]
  value?: number
  onChange?: (value: number) => void
  placeholder?: string
  /** onlyAvailable 仅展示可借设备；默认展示全部并禁用不可用项。 */
  onlyAvailable?: boolean
  /**
   * selectable 自定义某项是否可选（如维护登记允许使用中/维护中的设备，仅禁用丢失/报废）。
   * 未提供时仅“可借”状态可选。
   */
  selectable?: (equipment: Equipment) => boolean
  allowClear?: boolean
  disabled?: boolean
}

export function EquipmentSelect({
  equipment,
  value,
  onChange,
  placeholder = '选择设备',
  onlyAvailable = false,
  selectable,
  allowClear,
  disabled
}: EquipmentSelectProps) {
  const canSelect = selectable || ((item: Equipment) => isEquipmentAvailable(item.status))
  const list = onlyAvailable ? equipment.filter((item) => isEquipmentAvailable(item.status)) : equipment
  return (
    <Select
      style={{ width: '100%' }}
      placeholder={placeholder}
      value={value}
      allowClear={allowClear}
      disabled={disabled}
      onChange={onChange}
      options={list.map((item) => ({
        label: `${item.name}（${item.code}）- ${equipmentStatusText(item.status)}`,
        value: item.id,
        disabled: !canSelect(item)
      }))}
    />
  )
}

export default EquipmentSelect
