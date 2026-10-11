//go:build integration

package repository

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func opsRoutingStorageValue[T any](value T) *T { return &value }

func opsRoutingStorageInput(t *testing.T, diagnostics *service.RoutingDiagnostics) *service.OpsInsertErrorLogInput {
	t.Helper()
	input := &service.OpsInsertErrorLogInput{
		RequestID: "routing-storage-" + uuid.NewString(), ErrorPhase: "routing", ErrorType: "no_available_accounts",
		Severity: "error", StatusCode: 503, ErrorMessage: "真实选择失败", ErrorBody: "已脱敏故障正文",
		ErrorOwner: "platform", ErrorSource: "gateway", CreatedAt: time.Now().UTC(), RoutingDiagnostics: diagnostics,
	}
	if diagnostics != nil {
		raw, err := json.Marshal(diagnostics)
		require.NoError(t, err)
		input.RoutingDiagnosticsJSON = opsRoutingStorageValue(string(raw))
	}
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(context.Background(), "DELETE FROM ops_error_logs WHERE request_id = $1", input.RequestID)
		require.NoError(t, err)
	})
	return input
}

func opsRoutingStorageRequireDetail(t *testing.T, repo service.OpsRepository, id int64, input *service.OpsInsertErrorLogInput, diagnostics *service.RoutingDiagnostics) {
	t.Helper()
	got, err := repo.GetErrorLogByID(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, diagnostics, got.RoutingDiagnostics)
	require.Equal(t, input.ErrorPhase, got.Phase)
	require.Equal(t, input.ErrorType, got.Type)
	require.Equal(t, input.ErrorMessage, got.Message)
	require.Equal(t, input.ErrorBody, got.ErrorBody)
	require.Equal(t, input.StatusCode, got.StatusCode)
	require.Equal(t, input.ErrorOwner, got.Owner)
	require.Equal(t, input.ErrorSource, got.Source)
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	var decoded service.OpsErrorLogDetail
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Equal(t, diagnostics, decoded.RoutingDiagnostics)
	if diagnostics == nil {
		require.NotContains(t, string(raw), "routing_diagnostics")
	}
}

func TestOpsRoutingDiagnostics_SingleAndBatchPersistence(t *testing.T) {
	ctx := context.Background()
	repo := NewOpsRepository(integrationDB)
	empty := &service.RoutingDiagnostics{
		SchemaVersion: 1, SelectionAttempt: 1, SelectionLayer: opsRoutingStorageValue("load_balance"),
		SelectionReason: opsRoutingStorageValue("pool_empty"), CandidatePool: opsRoutingStorageValue(0),
		FilteredCandidates: opsRoutingStorageValue(0), FilterReasons: map[string]int{}, FilterCoverage: "complete",
	}
	partial := &service.RoutingDiagnostics{
		SchemaVersion: 1, Turn: opsRoutingStorageValue(2), SelectionAttempt: 2,
		SelectionLayer: opsRoutingStorageValue("load_balance"), SelectionReason: opsRoutingStorageValue("selection_exhausted"),
		CandidatePool: opsRoutingStorageValue(4), FilteredCandidates: opsRoutingStorageValue(1),
		FilterReasons: map[string]int{"model_not_supported": 1}, FilterCoverage: "partial",
	}
	unobserved := &service.RoutingDiagnostics{
		SchemaVersion: 1, SelectionAttempt: 3, SelectionLayer: opsRoutingStorageValue("channel_pricing"),
		SelectionReason: opsRoutingStorageValue("channel_pricing_restricted"), FilterCoverage: "unobserved",
	}
	single := opsRoutingStorageInput(t, empty)
	id, err := repo.InsertErrorLog(ctx, single)
	require.NoError(t, err)
	opsRoutingStorageRequireDetail(t, repo, id, single, empty)
	var stored string
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT routing_diagnostics::text FROM ops_error_logs WHERE id = $1", id).Scan(&stored))
	require.JSONEq(t, *single.RoutingDiagnosticsJSON, stored, "零与空映射完整落库")

	// 持久化权威来源是服务已冻结的序列化字段，不能借输入对象或当前请求重建。
	batchInputs := []*service.OpsInsertErrorLogInput{opsRoutingStorageInput(t, partial), opsRoutingStorageInput(t, unobserved), opsRoutingStorageInput(t, nil)}
	batchInputs[0].RoutingDiagnostics = empty.Clone()
	inserted, err := repo.BatchInsertErrorLogs(ctx, append([]*service.OpsInsertErrorLogInput{nil}, batchInputs...))
	require.NoError(t, err)
	require.EqualValues(t, 3, inserted)
	for i, expected := range []*service.RoutingDiagnostics{partial, unobserved, nil} {
		input := batchInputs[i]
		var id int64
		require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT id FROM ops_error_logs WHERE request_id = $1", input.RequestID).Scan(&id))
		opsRoutingStorageRequireDetail(t, repo, id, input, expected)
	}

	list, err := repo.ListErrorLogs(ctx, &service.OpsErrorLogFilter{RequestID: single.RequestID})
	require.NoError(t, err)
	require.Len(t, list.Errors, 1)
	listJSON, err := json.Marshal(list)
	require.NoError(t, err)
	require.NotContains(t, string(listJSON), "routing_diagnostics")
}

func TestOpsRoutingDiagnostics_HistoricalNullAndInvalidObject(t *testing.T) {
	ctx := context.Background()
	repo := NewOpsRepository(integrationDB)
	var signals bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&signals, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	for _, tc := range []struct {
		name    string
		raw     *string
		invalid bool
	}{
		{name: "legacy SQL NULL"},
		{name: "JSON null", raw: opsRoutingStorageValue("null")},
		{name: "unknown field", invalid: true, raw: opsRoutingStorageValue(`{"schema_version":1,"turn":null,"selection_attempt":1,"selection_layer":null,"selection_reason":null,"candidate_pool":null,"filtered_candidates":null,"filter_reasons":null,"filter_coverage":"unobserved","credentials":"private-diagnostic-payload"}`)},
		{name: "unknown version", invalid: true, raw: opsRoutingStorageValue(`{"schema_version":2,"turn":null,"selection_attempt":1,"selection_layer":null,"selection_reason":null,"candidate_pool":null,"filtered_candidates":null,"filter_reasons":null,"filter_coverage":"unobserved"}`)},
		{name: "fraction", invalid: true, raw: opsRoutingStorageValue(`{"schema_version":1,"turn":null,"selection_attempt":1.5,"selection_layer":null,"selection_reason":null,"candidate_pool":null,"filtered_candidates":null,"filter_reasons":null,"filter_coverage":"unobserved"}`)},
		{name: "missing fixed fields", invalid: true, raw: opsRoutingStorageValue(`{"schema_version":1,"selection_attempt":1,"filter_coverage":"unobserved"}`)},
		{name: "invalid relation", invalid: true, raw: opsRoutingStorageValue(`{"schema_version":1,"turn":null,"selection_attempt":1,"selection_layer":"load_balance","selection_reason":"pool_empty","candidate_pool":4,"filtered_candidates":0,"filter_reasons":{},"filter_coverage":"complete"}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := opsRoutingStorageInput(t, nil)
			// 直接模拟旧写入者及已损坏的历史对象，不经过本次服务接收校验。
			var id int64
			err := integrationDB.QueryRowContext(ctx, `INSERT INTO ops_error_logs
(request_id,error_phase,error_type,severity,status_code,error_message,error_body,error_owner,error_source,created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
				input.RequestID, input.ErrorPhase, input.ErrorType, input.Severity, input.StatusCode, input.ErrorMessage, input.ErrorBody, input.ErrorOwner, input.ErrorSource, input.CreatedAt).Scan(&id)
			require.NoError(t, err)
			if tc.raw != nil {
				_, err = integrationDB.ExecContext(ctx, "UPDATE ops_error_logs SET routing_diagnostics = $2::jsonb WHERE id = $1", id, *tc.raw)
				require.NoError(t, err)
			}
			signals.Reset()
			opsRoutingStorageRequireDetail(t, repo, id, input, nil)
			if tc.invalid {
				require.Contains(t, signals.String(), "routing_diagnostics_invalid")
				require.NotContains(t, signals.String(), "private-diagnostic-payload")
				require.NotContains(t, signals.String(), "credentials")
				require.NotContains(t, signals.String(), *tc.raw)
			} else {
				require.Empty(t, signals.String(), "未知诊断不产生损坏信号")
			}
		})
	}
}

func TestOpsRoutingDiagnostics_BatchFailureRemainsAtomic(t *testing.T) {
	ctx := context.Background()
	repo := NewOpsRepository(integrationDB)
	good := opsRoutingStorageInput(t, nil)
	bad := opsRoutingStorageInput(t, nil)
	bad.RoutingDiagnosticsJSON = opsRoutingStorageValue("{invalid-json")
	_, err := repo.BatchInsertErrorLogs(ctx, []*service.OpsInsertErrorLogInput{good, bad})
	require.Error(t, err, "数据库失败保留显式错误")
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM ops_error_logs WHERE request_id IN ($1, $2)", good.RequestID, bad.RequestID).Scan(&count))
	require.Zero(t, count, "同批失败不能留下部分写入")
}

func TestMigration252_RoutingDiagnosticsHistoricalAndIdempotent(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	_, err := tx.ExecContext(ctx, "CREATE TEMP TABLE ops_error_logs (id BIGINT, error_message TEXT)")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "INSERT INTO ops_error_logs (id, error_message) VALUES (1, '迁移前故障')")
	require.NoError(t, err)
	migration, err := dbmigrations.FS.ReadFile("252_add_ops_routing_diagnostics.sql")
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		_, err = tx.ExecContext(ctx, string(migration))
		require.NoError(t, err, "新增迁移可幂等执行")
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO ops_error_logs (id, error_message) VALUES (2, '扩展后旧写入故障')")
	require.NoError(t, err)
	for id, expected := range map[int]string{1: "迁移前故障", 2: "扩展后旧写入故障"} {
		var message string
		var diagnostics sql.NullString
		require.NoError(t, tx.QueryRowContext(ctx, "SELECT error_message, routing_diagnostics::text FROM ops_error_logs WHERE id = $1", id).Scan(&message, &diagnostics))
		require.Equal(t, expected, message)
		require.False(t, diagnostics.Valid, "历史与旧写入保持整体未知")
	}
	var dataType string
	var notNull bool
	var defaultExpr sql.NullString
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT format_type(a.atttypid, a.atttypmod), a.attnotnull, pg_get_expr(d.adbin, d.adrelid)
FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
WHERE a.attrelid = 'ops_error_logs'::regclass AND a.attname = 'routing_diagnostics'`).Scan(&dataType, &notNull, &defaultExpr))
	require.Equal(t, "jsonb", strings.ToLower(dataType))
	require.False(t, notNull)
	require.False(t, defaultExpr.Valid, "不能引入默认空对象或零统计")
}
