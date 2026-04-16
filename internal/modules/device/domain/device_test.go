package domain_test

import (
	"testing"

	"github.com/vvs/isp/internal/modules/device/domain"
)

func newTestDevice(t *testing.T) *domain.Device {
	t.Helper()
	d, err := domain.NewDevice("id-1", "Test Device", domain.TypeModem, "SN-001")
	if err != nil {
		t.Fatalf("NewDevice: %v", err)
	}
	return d
}

func TestNewDevice_RequiresName(t *testing.T) {
	_, err := domain.NewDevice("id-1", "", domain.TypeModem, "")
	if err != domain.ErrNameRequired {
		t.Fatalf("want ErrNameRequired, got %v", err)
	}
}

func TestNewDevice_DefaultsToInStock(t *testing.T) {
	d := newTestDevice(t)
	if d.Status != domain.StatusInStock {
		t.Fatalf("want in_stock, got %s", d.Status)
	}
}

func TestDeploy_FromInStock(t *testing.T) {
	d := newTestDevice(t)
	if err := d.Deploy("cust-1", "Site A"); err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if d.Status != domain.StatusDeployed {
		t.Fatalf("want deployed, got %s", d.Status)
	}
	if d.CustomerID != "cust-1" {
		t.Fatalf("want cust-1, got %s", d.CustomerID)
	}
}

func TestDeploy_FromDeployed_Fails(t *testing.T) {
	d := newTestDevice(t)
	_ = d.Deploy("cust-1", "")
	if err := d.Deploy("cust-2", ""); err != domain.ErrInvalidTransition {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
}

func TestReturn_FromDeployed(t *testing.T) {
	d := newTestDevice(t)
	_ = d.Deploy("cust-1", "Site A")
	if err := d.Return(); err != nil {
		t.Fatalf("Return: %v", err)
	}
	if d.Status != domain.StatusInStock {
		t.Fatalf("want in_stock, got %s", d.Status)
	}
	if d.CustomerID != "" {
		t.Fatalf("CustomerID should be cleared")
	}
}

func TestReturn_FromInStock_Fails(t *testing.T) {
	d := newTestDevice(t)
	if err := d.Return(); err != domain.ErrInvalidTransition {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
}

func TestDecommission_FromInStock(t *testing.T) {
	d := newTestDevice(t)
	if err := d.Decommission(); err != nil {
		t.Fatalf("Decommission: %v", err)
	}
	if d.Status != domain.StatusDecommissioned {
		t.Fatalf("want decommissioned, got %s", d.Status)
	}
}

func TestDecommission_FromDeployed(t *testing.T) {
	d := newTestDevice(t)
	_ = d.Deploy("cust-1", "")
	if err := d.Decommission(); err != nil {
		t.Fatalf("Decommission from deployed: %v", err)
	}
	if d.CustomerID != "" {
		t.Fatalf("CustomerID should be cleared on decommission")
	}
}

func TestDecommission_Twice_Fails(t *testing.T) {
	d := newTestDevice(t)
	_ = d.Decommission()
	if err := d.Decommission(); err != domain.ErrInvalidTransition {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
}
