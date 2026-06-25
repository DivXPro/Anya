package store

import "testing"

func TestAuthorizeDevicePersists(t *testing.T) {
	db := testDB(t)

	if auth, err := IsDeviceAuthorized(db, "dev-1"); err != nil {
		t.Fatalf("IsDeviceAuthorized error: %v", err)
	} else if auth {
		t.Fatal("dev-1 should not be authorized initially")
	}

	if err := AuthorizeDevice(db, "dev-1", "Elf Stick"); err != nil {
		t.Fatalf("AuthorizeDevice error: %v", err)
	}

	if auth, err := IsDeviceAuthorized(db, "dev-1"); err != nil {
		t.Fatalf("IsDeviceAuthorized error after auth: %v", err)
	} else if !auth {
		t.Fatal("dev-1 should be authorized")
	}

	// Re-authorizing with an empty name must not delete the record.
	if err := AuthorizeDevice(db, "dev-1", ""); err != nil {
		t.Fatalf("re-authorize error: %v", err)
	}
	if auth, err := IsDeviceAuthorized(db, "dev-1"); err != nil {
		t.Fatalf("IsDeviceAuthorized error after re-auth: %v", err)
	} else if !auth {
		t.Fatal("dev-1 should still be authorized after re-auth with empty name")
	}

	list, err := ListAuthorizedDevices(db)
	if err != nil {
		t.Fatalf("ListAuthorizedDevices error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 authorized device, got %d", len(list))
	}
	if list[0].Revoked {
		t.Fatal("device should not be revoked")
	}
}

func TestRevokeDevice(t *testing.T) {
	db := testDB(t)
	if err := AuthorizeDevice(db, "dev-2", "Elf Stick 2"); err != nil {
		t.Fatalf("AuthorizeDevice error: %v", err)
	}
	if err := RevokeDevice(db, "dev-2"); err != nil {
		t.Fatalf("RevokeDevice error: %v", err)
	}
	if auth, err := IsDeviceAuthorized(db, "dev-2"); err != nil {
		t.Fatalf("IsDeviceAuthorized error: %v", err)
	} else if auth {
		t.Fatal("dev-2 should not be authorized after revoke")
	}
}
