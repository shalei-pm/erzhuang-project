package auditlog

import (
	"context"
	"database/sql"
	"testing"
)

type recorderStub struct {
	event AuditEvent
}

func (s *recorderStub) RecordAudit(_ context.Context, event AuditEvent) error {
	s.event = event
	return nil
}

type txRecorderStub struct {
	recorderStub
	tx *sql.Tx
}

func (s *txRecorderStub) RecorderForTx(tx *sql.Tx) AuditRecorder {
	s.tx = tx
	return s
}

func TestRecorderContractsAcceptEventAndTransactionBoundRecorder(t *testing.T) {
	var recorder AuditRecorder = &recorderStub{}
	var txRecorder TxRecorder = &txRecorderStub{}

	event := AuditEvent{Action: "store.update"}
	if err := recorder.RecordAudit(context.Background(), event); err != nil {
		t.Fatalf("record audit event: %v", err)
	}
	if got := txRecorder.RecorderForTx(nil); got == nil {
		t.Fatal("transaction recorder must return an audit recorder")
	}
}
