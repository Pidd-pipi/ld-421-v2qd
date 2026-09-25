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

// Create 创建维护计划/记录。设备登记维护后立即转入“维护中”，停止借用与预约。
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
	switch equipment.Status {
	case constants.AssetStatusMaintenance:
		// 已在维护中，允许追加维护记录。
	case constants.AssetStatusLost:
		return nil, apperrors.NewBusinessError(40900, 409, "设备已丢失，无法登记维护")
	case constants.AssetStatusRetired:
		return nil, apperrors.NewBusinessError(40900, 409, "设备已报废，无法登记维护")
	case constants.AssetStatusInUse:
		return nil, apperrors.NewBusinessError(40900, 409, "设备正在借用中，请先归还再登记维护")
	}
	record.Result = constants.MaintenanceResultPending
	if err := s.repo.Create(ctx, record); err != nil {
		return nil, fmt.Errorf("create maintenance record: %w", err)
	}
	if equipment.Status != constants.AssetStatusMaintenance {
		equipment.Status = constants.AssetStatusMaintenance
		if err := s.equipmentRepo.Update(ctx, equipment); err != nil {
			return nil, fmt.Errorf("mark equipment maintenance: %w", err)
		}
	}
	if err := s.audit.Log(ctx, actor, "maintenance.create", "maintenance", record.ID, fmt.Sprintf("创建维护记录 %d，设备 %d 转入维护中", record.ID, record.EquipmentID)); err != nil {
		return nil, err
	}
	return record, nil
}

// Execute 执行维护并记录结果：通过恢复可借，失败或需跟进继续停留在维护中。
func (s *MaintenanceService) Execute(ctx context.Context, id uint, result constants.MaintenanceResult, actor Actor) error {
	record, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.mapNotFound(err)
	}
	if record.Result != constants.MaintenanceResultPending {
		return apperrors.NewBusinessError(40900, 409, "该维护记录已执行，不可重复登记结果")
	}
	if !result.IsFinal() {
		return apperrors.NewBusinessError(40000, 400, "维护结果无效")
	}
	record.Result = result
	if err := s.repo.Update(ctx, record); err != nil {
		return fmt.Errorf("execute maintenance record: %w", err)
	}
	equipment, err := s.equipmentRepo.FindByID(ctx, record.EquipmentID)
	if err != nil {
		return fmt.Errorf("find equipment: %w", err)
	}
	if result == constants.MaintenanceResultPass && equipment.Status == constants.AssetStatusMaintenance {
		remaining, err := s.repo.CountPending(ctx, record.EquipmentID, record.ID)
		if err != nil {
			return err
		}
		if remaining == 0 {
			equipment.Status = constants.AssetStatusAvailable
			if err := s.equipmentRepo.Update(ctx, equipment); err != nil {
				return fmt.Errorf("restore equipment available: %w", err)
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
