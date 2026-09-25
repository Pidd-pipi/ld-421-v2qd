package service

import (
	"context"
	"testing"
	"time"

	"github.com/labequipment/lab-equipment/internal/constants"
	"github.com/labequipment/lab-equipment/internal/model"
)

func borrowAndApprove(t *testing.T, env *testEnv, condition constants.ReturnCondition) {
	t.Helper()
	ctx := context.Background()
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "LabManager"}
	record, err := env.borrowService.Create(ctx, &model.BorrowRecord{
		EquipmentID:        1,
		BorrowDate:         time.Now(),
		ExpectedReturnDate: time.Now().AddDate(0, 0, 7),
	}, actor)
	if err != nil {
		t.Fatalf("create borrow: %v", err)
	}
	if err := env.borrowService.Approve(ctx, record.ID, actor); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if err := env.borrowService.Return(ctx, record.ID, time.Now(), condition, actor); err != nil {
		t.Fatalf("return: %v", err)
	}
}

// 良好归还后设备恢复可借。
func TestBorrowService_ReturnGoodRestoresAvailable(t *testing.T) {
	env := newTestEnv(t)
	borrowAndApprove(t, env, constants.ReturnConditionGood)
	equipment, err := env.equipmentRepo.FindByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("find equipment: %v", err)
	}
	if equipment.Status != constants.AssetStatusAvailable {
		t.Fatalf("expected available after good return, got %s", equipment.Status)
	}
}

// 损坏归还后设备转入维护中，不可再借用。
func TestBorrowService_ReturnDamagedEntersMaintenance(t *testing.T) {
	env := newTestEnv(t)
	borrowAndApprove(t, env, constants.ReturnConditionDamaged)
	ctx := context.Background()
	equipment, err := env.equipmentRepo.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("find equipment: %v", err)
	}
	if equipment.Status != constants.AssetStatusMaintenance {
		t.Fatalf("expected maintenance after damaged return, got %s", equipment.Status)
	}
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	if _, err := env.borrowService.Create(ctx, &model.BorrowRecord{
		EquipmentID:        1,
		BorrowDate:         time.Now(),
		ExpectedReturnDate: time.Now().AddDate(0, 0, 3),
	}, actor); err == nil {
		t.Fatalf("expected borrow to be rejected while in maintenance")
	}
}

// 丢失归还后设备标记为丢失，借用与预约均不可用。
func TestBorrowService_ReturnLostMarksLost(t *testing.T) {
	env := newTestEnv(t)
	borrowAndApprove(t, env, constants.ReturnConditionLost)
	ctx := context.Background()
	equipment, err := env.equipmentRepo.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("find equipment: %v", err)
	}
	if equipment.Status != constants.AssetStatusLost {
		t.Fatalf("expected lost, got %s", equipment.Status)
	}
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	if _, err := env.borrowService.Create(ctx, &model.BorrowRecord{
		EquipmentID:        1,
		BorrowDate:         time.Now(),
		ExpectedReturnDate: time.Now().AddDate(0, 0, 3),
	}, actor); err == nil {
		t.Fatalf("expected borrow to be rejected for lost equipment")
	}
	start := time.Now().Add(24 * time.Hour)
	if _, err := env.reservationService.Create(ctx, &model.Reservation{
		EquipmentID: 1,
		StartTime:   start,
		EndTime:     start.Add(2 * time.Hour),
		Purpose:     "预约丢失设备",
	}, actor); err == nil {
		t.Fatalf("expected reservation to be rejected for lost equipment")
	}
}
