package service

import (
	"context"
	"testing"
	"time"

	"github.com/labequipment/lab-equipment/internal/constants"
	"github.com/labequipment/lab-equipment/internal/model"
)

func createPendingMaintenance(t *testing.T, env *testEnv) *model.MaintenanceRecord {
	t.Helper()
	ctx := context.Background()
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "LabManager"}
	record, err := env.maintenanceService.Create(ctx, &model.MaintenanceRecord{
		EquipmentID:     1,
		Type:            constants.MaintenanceTypeCorrective,
		Content:         "检修",
		MaintenanceDate: time.Now(),
		MaintainerID:    env.ownerID,
	}, actor)
	if err != nil {
		t.Fatalf("create maintenance: %v", err)
	}
	return record
}

// 创建维护后设备进入维护中，借用与预约被拒绝。
func TestMaintenanceService_CreateEntersMaintenance(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	createPendingMaintenance(t, env)

	equipment, err := env.equipmentRepo.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("find equipment: %v", err)
	}
	if equipment.Status != constants.AssetStatusMaintenance {
		t.Fatalf("expected maintenance, got %s", equipment.Status)
	}
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	if _, err := env.borrowService.Create(ctx, &model.BorrowRecord{
		EquipmentID:        1,
		BorrowDate:         time.Now(),
		ExpectedReturnDate: time.Now().AddDate(0, 0, 3),
	}, actor); err == nil {
		t.Fatalf("expected borrow rejected in maintenance")
	}
	start := time.Now().Add(24 * time.Hour)
	if _, err := env.reservationService.Create(ctx, &model.Reservation{
		EquipmentID: 1,
		StartTime:   start,
		EndTime:     start.Add(2 * time.Hour),
		Purpose:     "预约维护中设备",
	}, actor); err == nil {
		t.Fatalf("expected reservation rejected in maintenance")
	}
}

// 维护通过：设备恢复可借。
func TestMaintenanceService_ExecutePassRestoresAvailable(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	record := createPendingMaintenance(t, env)
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "LabManager"}
	if err := env.maintenanceService.Execute(ctx, record.ID, constants.MaintenanceResultPass, actor); err != nil {
		t.Fatalf("execute: %v", err)
	}
	equipment, _ := env.equipmentRepo.FindByID(ctx, 1)
	if equipment.Status != constants.AssetStatusAvailable {
		t.Fatalf("expected available after pass, got %s", equipment.Status)
	}
}

// 维护失败或需跟进：设备继续停留在维护中，且不可重复登记结果。
func TestMaintenanceService_ExecuteFailKeepsMaintenance(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	record := createPendingMaintenance(t, env)
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "LabManager"}

	if err := env.maintenanceService.Execute(ctx, record.ID, constants.MaintenanceResultFail, actor); err != nil {
		t.Fatalf("execute fail: %v", err)
	}
	equipment, _ := env.equipmentRepo.FindByID(ctx, 1)
	if equipment.Status != constants.AssetStatusMaintenance {
		t.Fatalf("expected still maintenance after fail, got %s", equipment.Status)
	}
	if err := env.maintenanceService.Execute(ctx, record.ID, constants.MaintenanceResultPass, actor); err == nil {
		t.Fatalf("expected re-execute to be rejected")
	}

	record2 := createPendingMaintenance(t, env)
	if err := env.maintenanceService.Execute(ctx, record2.ID, constants.MaintenanceResultNeedsFollowUp, actor); err != nil {
		t.Fatalf("execute needs follow up: %v", err)
	}
	equipment, _ = env.equipmentRepo.FindByID(ctx, 1)
	if equipment.Status != constants.AssetStatusMaintenance {
		t.Fatalf("expected still maintenance after needs follow up, got %s", equipment.Status)
	}
}
