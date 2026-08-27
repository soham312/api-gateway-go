package health

import "testing"

func TestNewBackend_DefaultsWeight(t *testing.T) {
	b := NewBackend("http://b1", 0)
	if b.Weight != 1 {
		t.Errorf("expected default weight 1, got %d", b.Weight)
	}
	b = NewBackend("http://b1", -5)
	if b.Weight != 1 {
		t.Errorf("expected default weight 1 for negative input, got %d", b.Weight)
	}
}

func TestBackend_InitiallyHealthy(t *testing.T) {
	b := NewBackend("http://b1", 1)
	if !b.IsHealthy() {
		t.Errorf("expected new backend to be healthy")
	}
	if b.GetState() != StateClosed {
		t.Errorf("expected initial state Closed, got %v", b.GetState())
	}
}

func TestBackend_ConnectionCounting(t *testing.T) {
	b := NewBackend("http://b1", 1)
	b.RecordConnection()
	b.RecordConnection()
	if b.ActiveConnections != 2 {
		t.Errorf("expected 2 active connections, got %d", b.ActiveConnections)
	}
	b.ReleaseConnection()
	if b.ActiveConnections != 1 {
		t.Errorf("expected 1 active connection after release, got %d", b.ActiveConnections)
	}
}

func TestBackend_CircuitTripsAfterDefaultFailureThreshold(t *testing.T) {
	b := NewBackend("http://b1", 1)
	// default maxFailures is 3 when no config is loaded
	b.RecordFailure()
	b.RecordFailure()
	if b.GetState() != StateClosed {
		t.Fatalf("expected circuit to remain closed before threshold, got %v", b.GetState())
	}
	b.RecordFailure()
	if b.GetState() != StateOpen {
		t.Errorf("expected circuit to open after 3 failures, got %v", b.GetState())
	}
	if b.IsHealthy() {
		t.Errorf("expected backend to be unhealthy once circuit is open")
	}
}

func TestBackend_HalfOpenFailureReturnsToOpen(t *testing.T) {
	b := NewBackend("http://b1", 1)
	b.SetState(StateHalfOpen)
	b.RecordFailure()
	if b.GetState() != StateOpen {
		t.Errorf("expected half-open failure to trip circuit back open, got %v", b.GetState())
	}
}

func TestBackend_HalfOpenSuccessCloses(t *testing.T) {
	b := NewBackend("http://b1", 1)
	b.SetState(StateHalfOpen)
	// default successThreshold is 1 when no config is loaded
	b.RecordSuccess()
	if b.GetState() != StateClosed {
		t.Errorf("expected half-open success to close circuit, got %v", b.GetState())
	}
	if !b.IsHealthy() {
		t.Errorf("expected backend to be healthy after circuit closes")
	}
}

func TestBackend_SuccessResetsFailureCount(t *testing.T) {
	b := NewBackend("http://b1", 1)
	b.RecordFailure()
	b.RecordFailure()
	b.RecordSuccess()
	// after a success, failures reset to 0, so it should take 3 more failures to trip
	b.RecordFailure()
	b.RecordFailure()
	if b.GetState() != StateClosed {
		t.Fatalf("expected circuit closed after failure count reset, got %v", b.GetState())
	}
	b.RecordFailure()
	if b.GetState() != StateOpen {
		t.Errorf("expected circuit open after 3 fresh failures, got %v", b.GetState())
	}
}
