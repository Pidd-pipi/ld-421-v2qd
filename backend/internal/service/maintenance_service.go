package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/labequipment/lab-equipment/internal/constants"
	apperrors "github.com/labequipment/lab-equipment/internal/errors"
	"github.com/labequipment/lab-equipment/internal/model"
	"github.com/labequipment/lab-equipment/internal/repository"
)

// MaintenanceService 维护记录业务服务。
type MaintenanceService struct {
	repo          repository.MaintenanceRepository
	equipmentRepo repository.EquipmentRepository
	audit         *AuditService
	logger        *slog.Logger
}

// NewMaintenanceService 构造维护服务。
func NewMaintenanceService(
	repo repository.MaintenanceRepository,
	equipmentRepo repository.EquipmentRepository,
	audit *AuditService,
	logger *slog.Logger,
) *MaintenanceService {
	return &MaintenanceService{repo: repo, equipmentRepo: equipmentRepo, audit: audit, logger: logger}
}

// Create 创建维护计划/记录。
func (s *MaintenanceService) Create(ctx context.Context, record *model.MaintenanceRecord, actor Actor) (*model.MaintenanceRecord, error) {
	if !record.Type.Valid() {
		return nil, apperrors.NewBusinessError(40000, 400, "维护类型无效")
	}
	equipment, err := s.equipmentRepo.FindByID(ctx, record.EquipmentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperrors.NewBusinessError(40400, 404, "设备不存在")
		}
		return nil, fmt.Errorf("find equipment: %w", err)
	}
	// 已丢失/报废设备属于终态，不再登记维护。
	if equipment.Status == constants.AssetStatusLost {
		return nil, apperrors.NewBusinessError(40900, 409, "设备已丢失，无法登记维护")
	}
	if equipment.Status == constants.AssetStatusRetired {
		return nil, apperrors.NewBusinessError(40900, 409, "设备已报废，无法登记维护")
	}
	record.Result = constants.MaintenanceResultNeedsFollowUp
	if err := s.repo.Create(ctx, record); err != nil {
		return nil, fmt.Errorf("create maintenance record: %w", err)
	}
	// 登记维护期间设备不可借用/预约，维护结果决定后续能否恢复可借。
	if equipment.Status != constants.AssetStatusMaintenance {
		equipment.Status = constants.AssetStatusMaintenance
		if err := s.equipmentRepo.Update(ctx, equipment); err != nil {
			return nil, fmt.Errorf("mark equipment maintenance: %w", err)
		}
	}
	if err := s.audit.Log(ctx, actor, "maintenance.create", "maintenance", record.ID, fmt.Sprintf("创建维护记录 %d，设备 %d 转入维护", record.ID, record.EquipmentID)); err != nil {
		return nil, err
	}
	return record, nil
}

// Execute 执行维护并记录结果。
func (s *MaintenanceService) Execute(ctx context.Context, id uint, result constants.MaintenanceResult, actor Actor) error {
	record, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.mapNotFound(err)
	}
	if !result.Valid() {
		return apperrors.NewBusinessError(40000, 400, "维护结果无效")
	}
	record.Result = result
	if err := s.repo.Update(ctx, record); err != nil {
		return fmt.Errorf("execute maintenance record: %w", err)
	}
	// 维护结果决定设备后续能否出借：
	// 通过则恢复可借；失败或需跟进则继续停留在维护状态。
	equipment, err := s.equipmentRepo.FindByID(ctx, record.EquipmentID)
	if err == nil {
		var nextStatus constants.AssetStatus
		switch result {
		case constants.MaintenanceResultPass:
			if equipment.Status == constants.AssetStatusMaintenance {
				nextStatus = constants.AssetStatusAvailable
			}
		case constants.MaintenanceResultFail, constants.MaintenanceResultNeedsFollowUp:
			// 丢失/报废为终态，维护结果不应将其改回维护。
			if equipment.Status != constants.AssetStatusLost && equipment.Status != constants.AssetStatusRetired {
				nextStatus = constants.AssetStatusMaintenance
			}
		}
		if nextStatus != "" && nextStatus != equipment.Status {
			equipment.Status = nextStatus
			if err := s.equipmentRepo.Update(ctx, equipment); err != nil {
				return fmt.Errorf("update equipment status after maintenance: %w", err)
			}
		}
	}
	if err := s.audit.Log(ctx, actor, "maintenance.execute", "maintenance", id, fmt.Sprintf("执行维护记录 %d，结果 %s", id, result)); err != nil {
		return err
	}
	return nil
}

// List 分页查询维护记录。
func (s *MaintenanceService) List(ctx context.Context, filter repository.MaintenanceFilter) ([]model.MaintenanceRecord, int64, error) {
	list, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list maintenance records: %w", err)
	}
	return list, total, nil
}

// Stats 返回维护统计。
func (s *MaintenanceService) Stats(ctx context.Context) (*repository.MaintenanceStats, error) {
	stats, err := s.repo.Stats(ctx)
	if err != nil {
		return nil, fmt.Errorf("maintenance stats: %w", err)
	}
	return stats, nil
}

func (s *MaintenanceService) mapNotFound(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return apperrors.NewBusinessError(40400, 404, "维护记录不存在")
	}
	return fmt.Errorf("find maintenance record: %w", err)
}
