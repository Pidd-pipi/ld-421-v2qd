package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/labequipment/lab-equipment/internal/constants"
	"github.com/labequipment/lab-equipment/internal/model"
)

func createApprovedBorrow(t *testing.T, env *testEnv, ctx context.Context, actor Actor) *model.BorrowRecord {
	t.Helper()
	record, err := env.borrowService.Create(ctx, &model.BorrowRecord{
		EquipmentID:        1,
		BorrowDate:         time.Now(),
		ExpectedReturnDate: time.Now().AddDate(0, 0, 7),
	}, actor)
	if err != nil {
		t.Fatalf("create borrow: %v", err)
	}
	if err := env.borrowService.Approve(ctx, record.ID, actor); err != nil {
		t.Fatalf("approve borrow: %v", err)
	}
	return record
}

func assertEquipmentStatus(t *testing.T, env *testEnv, want constants.AssetStatus) {
	t.Helper()
	equipment, err := env.equipmentRepo.FindByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("find equipment: %v", err)
	}
	if equipment.Status != want {
		t.Fatalf("expected equipment status %s, got %s", want, equipment.Status)
	}
}

// 良好归还后设备应恢复为可借。
func TestReturnGood_RestoresAvailable(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "LabManager"}
	record := createApprovedBorrow(t, env, ctx, actor)
	assertEquipmentStatus(t, env, constants.AssetStatusInUse)

	if err := env.borrowService.Return(ctx, record.ID, time.Now(), constants.ReturnConditionGood, actor); err != nil {
		t.Fatalf("return good: %v", err)
	}
	assertEquipmentStatus(t, env, constants.AssetStatusAvailable)
}

// 损坏归还应转入维护并生成一条需跟进的纠正性维护记录；
// 维护通过后恢复可借，失败或需跟进则继续停留在维护。
func TestReturnDamaged_EntersMaintenanceFlow(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "LabManager"}
	record := createApprovedBorrow(t, env, ctx, actor)

	if err := env.borrowService.Return(ctx, record.ID, time.Now(), constants.ReturnConditionDamaged, actor); err != nil {
		t.Fatalf("return damaged: %v", err)
	}
	assertEquipmentStatus(t, env, constants.AssetStatusMaintenance)

	var maintenances []model.MaintenanceRecord
	if err := env.db.Where("equipment_id = ?", 1).Find(&maintenances).Error; err != nil {
		t.Fatalf("query maintenances: %v", err)
	}
	if len(maintenances) != 1 {
		t.Fatalf("expected 1 corrective maintenance record, got %d", len(maintenances))
	}
	auto := maintenances[0]
	if auto.Type != constants.MaintenanceTypeCorrective || auto.Result != constants.MaintenanceResultNeedsFollowUp {
		t.Fatalf("expected corrective/needsFollowUp auto record, got %s/%s", auto.Type, auto.Result)
	}

	// 维护失败：继续维护。
	if err := env.maintenanceService.Execute(ctx, auto.ID, constants.MaintenanceResultFail, actor); err != nil {
		t.Fatalf("execute fail: %v", err)
	}
	assertEquipmentStatus(t, env, constants.AssetStatusMaintenance)

	// 需跟进：继续维护。
	if err := env.maintenanceService.Execute(ctx, auto.ID, constants.MaintenanceResultNeedsFollowUp, actor); err != nil {
		t.Fatalf("execute needs follow up: %v", err)
	}
	assertEquipmentStatus(t, env, constants.AssetStatusMaintenance)

	// 维护通过：恢复可借。
	if err := env.maintenanceService.Execute(ctx, auto.ID, constants.MaintenanceResultPass, actor); err != nil {
		t.Fatalf("execute pass: %v", err)
	}
	assertEquipmentStatus(t, env, constants.AssetStatusAvailable)
}

// 丢失归还应标记丢失，且借用与预约均被明确拒绝。
func TestReturnLost_BlocksBorrowAndReservation(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "LabManager"}
	record := createApprovedBorrow(t, env, ctx, actor)

	if err := env.borrowService.Return(ctx, record.ID, time.Now(), constants.ReturnConditionLost, actor); err != nil {
		t.Fatalf("return lost: %v", err)
	}
	assertEquipmentStatus(t, env, constants.AssetStatusLost)

	_, err := env.borrowService.Create(ctx, &model.BorrowRecord{
		EquipmentID:        1,
		BorrowDate:         time.Now(),
		ExpectedReturnDate: time.Now().AddDate(0, 0, 1),
	}, actor)
	if err == nil || !strings.Contains(err.Error(), "已丢失") {
		t.Fatalf("expected lost borrow rejection, got %v", err)
	}

	start := time.Now().Add(24 * time.Hour)
	_, err = env.reservationService.Create(ctx, &model.Reservation{
		EquipmentID: 1,
		StartTime:   start,
		EndTime:     start.Add(time.Hour),
	}, actor)
	if err == nil || !strings.Contains(err.Error(), "已丢失") {
		t.Fatalf("expected lost reservation rejection, got %v", err)
	}
}

// 维护中的设备借用与预约都应被明确拒绝。
func TestMaintenance_BlocksBorrowAndReservation(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	equipment, _ := env.equipmentRepo.FindByID(ctx, 1)
	equipment.Status = constants.AssetStatusMaintenance
	if err := env.equipmentRepo.Update(ctx, equipment); err != nil {
		t.Fatalf("set maintenance: %v", err)
	}

	_, err := env.borrowService.Create(ctx, &model.BorrowRecord{
		EquipmentID:        1,
		BorrowDate:         time.Now(),
		ExpectedReturnDate: time.Now().AddDate(0, 0, 1),
	}, actor)
	if err == nil || !strings.Contains(err.Error(), "维护中") {
		t.Fatalf("expected maintenance borrow rejection, got %v", err)
	}

	start := time.Now().Add(24 * time.Hour)
	_, err = env.reservationService.Create(ctx, &model.Reservation{
		EquipmentID: 1,
		StartTime:   start,
		EndTime:     start.Add(time.Hour),
	}, actor)
	if err == nil || !strings.Contains(err.Error(), "维护中") {
		t.Fatalf("expected maintenance reservation rejection, got %v", err)
	}
}

// 丢失属于终态：对其维护记录登记结果不应把设备改回可借或维护。
func TestMaintenanceResult_DoesNotReviveLostEquipment(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "LabManager"}

	maintenance := &model.MaintenanceRecord{
		EquipmentID:     1,
		Type:            constants.MaintenanceTypeCorrective,
		MaintenanceDate: time.Now(),
		MaintainerID:    env.ownerID,
		Result:          constants.MaintenanceResultNeedsFollowUp,
	}
	if err := env.db.Create(maintenance).Error; err != nil {
		t.Fatalf("create maintenance: %v", err)
	}
	equipment, _ := env.equipmentRepo.FindByID(ctx, 1)
	equipment.Status = constants.AssetStatusLost
	if err := env.equipmentRepo.Update(ctx, equipment); err != nil {
		t.Fatalf("set lost: %v", err)
	}

	if err := env.maintenanceService.Execute(ctx, maintenance.ID, constants.MaintenanceResultPass, actor); err != nil {
		t.Fatalf("execute pass: %v", err)
	}
	assertEquipmentStatus(t, env, constants.AssetStatusLost)
}

// 登记维护即把设备转入维护，维护通过后恢复可借。
func TestMaintenanceCreate_ParksEquipmentThenPassRestores(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "LabManager"}

	record, err := env.maintenanceService.Create(ctx, &model.MaintenanceRecord{
		EquipmentID:     1,
		Type:            constants.MaintenanceTypePreventive,
		MaintenanceDate: time.Now(),
		MaintainerID:    env.ownerID,
	}, actor)
	if err != nil {
		t.Fatalf("create maintenance: %v", err)
	}
	if record.Result != constants.MaintenanceResultNeedsFollowUp {
		t.Fatalf("new maintenance should await result, got %s", record.Result)
	}
	assertEquipmentStatus(t, env, constants.AssetStatusMaintenance)

	if err := env.maintenanceService.Execute(ctx, record.ID, constants.MaintenanceResultPass, actor); err != nil {
		t.Fatalf("execute pass: %v", err)
	}
	assertEquipmentStatus(t, env, constants.AssetStatusAvailable)
}

// 已丢失设备不允许登记维护。
func TestMaintenanceCreate_RejectsLostEquipment(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "LabManager"}
	equipment, _ := env.equipmentRepo.FindByID(ctx, 1)
	equipment.Status = constants.AssetStatusLost
	if err := env.equipmentRepo.Update(ctx, equipment); err != nil {
		t.Fatalf("set lost: %v", err)
	}

	_, err := env.maintenanceService.Create(ctx, &model.MaintenanceRecord{
		EquipmentID:     1,
		Type:            constants.MaintenanceTypeCorrective,
		MaintenanceDate: time.Now(),
		MaintainerID:    env.ownerID,
	}, actor)
	if err == nil || !strings.Contains(err.Error(), "已丢失") {
		t.Fatalf("expected lost maintenance rejection, got %v", err)
	}
}
