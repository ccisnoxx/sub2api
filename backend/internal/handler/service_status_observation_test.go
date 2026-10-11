//go:build unit

package handler

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestServiceStatusSourceHTTPRealRetryAndReusedClientHeader(t *testing.T) {
	usage := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 4)}
	first := codexWireAccount(901, "first", nil)
	second := codexWireAccount(902, "second", nil)
	upstream, router, cleanup := newCodexWireEntryWithUsage(t, []service.Account{first, second}, usage)
	defer cleanup()
	upstream.sequence = []int{429, 0}
	upstream.errorBody = `{"error":{"code":"rate_limit_exceeded","message":"upstream rejected"}}`
	rec := codexWireSend(t, router, "/v1/responses", codexWireResponsesBody(true))
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var firstLog *service.UsageLog
	select {
	case firstLog = <-usage.created:
	case <-time.After(time.Second):
		t.Fatal("未记录真实重试用量")
	}
	require.NotNil(t, firstLog.ServiceStatusObservation)
	require.Equal(t, "terminal", firstLog.ServiceStatusObservation.EventRole)
	require.Equal(t, service.CompletionStatusCompleted, *firstLog.ServiceStatusObservation.TerminalKind)
	require.Nil(t, firstLog.ServiceStatusObservation.LogicalTurn)
	// 相同真实客户端会话/请求头，下一次HTTP生命周期必须独立。
	rec = codexWireSend(t, router, "/v1/responses", codexWireResponsesBody(true))
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var next *service.UsageLog
	select {
	case next = <-usage.created:
	case <-time.After(time.Second):
		t.Fatal("未记录后续HTTP用量")
	}
	require.NotEqual(t, firstLog.ServiceStatusObservation.ObservationKey, next.ServiceStatusObservation.ObservationKey)
}

func TestServiceStatusSourceOpsQueueSnapshotAndDrop(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 1)
	ctx := service.WithServiceStatusRequest(context.Background(), 2)
	fact := service.ServiceStatusAttemptObservation(ctx, "provider_capacity", time.Now())
	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	entry := &service.OpsInsertErrorLogInput{ServiceStatusObservation: fact}
	enqueueOpsErrorLog(ops, entry)
	*fact.LogicalTurn = 99
	job := <-opsErrorLogQueue
	require.Equal(t, 2, *job.entry.ServiceStatusObservation.LogicalTurn)
	require.NotNil(t, job.entry.ServiceStatusObservationJSON)
	// 人工占满已有有界队列，验证真实丢弃分支报告源缺口。
	opsErrorLogQueue <- job
	before := time.Now()
	enqueueOpsErrorLog(ops, entry)
	require.True(t, service.ServiceStatusSourceErrorSince(before))
}

func TestServiceStatusSourceWSLogicalTurnAcrossLocalProxyReset(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/v1/responses", nil).WithContext(service.WithServiceStatusRequest(context.Background(), 0))
	turns := &openAIWSRoutingTurns{}
	turns.beginProxy(c)
	turns.beginTurn(c, 1)
	first := service.ServiceStatusAttemptObservation(c.Request.Context(), "", time.Now())
	turns.beginTurn(c, 2)
	second := service.ServiceStatusAttemptObservation(c.Request.Context(), "", time.Now())
	turns.beginProxy(c)
	turns.beginTurn(c, 1)
	retry := service.ServiceStatusAttemptObservation(c.Request.Context(), "", time.Now())
	require.NotEqual(t, first.ObservationKey, second.ObservationKey)
	require.Equal(t, second.ObservationKey, retry.ObservationKey)
	require.Equal(t, 2, *retry.LogicalTurn)
}

func TestServiceStatusSourceUsageTaskPanicIsObservable(t *testing.T) {
	ctx := service.WithServiceStatusRequest(context.Background(), 0)
	fact := service.ServiceStatusAttemptObservation(ctx, "", time.Now())
	h := &OpenAIGatewayHandler{}
	before := time.Now()
	h.submitOpenAIUsageRecordTask(ctx, &service.OpenAIForwardResult{UsageTiming: service.UsageTiming{ServiceStatusObservation: fact}}, func(context.Context) { panic("fixture") })
	require.True(t, service.ServiceStatusSourceErrorSince(before))
}

type serviceStatusPanicOpsRepository struct{ service.OpsRepository }

func (*serviceStatusPanicOpsRepository) BatchInsertErrorLogs(context.Context, []*service.OpsInsertErrorLogInput) (int64, error) {
	panic("S4 fixture")
}
func (*serviceStatusPanicOpsRepository) InsertErrorLog(context.Context, *service.OpsInsertErrorLogInput) (int64, error) {
	panic("S4 fixture")
}
func TestServiceStatusSourceOpsFlushPanicIsObservable(t *testing.T) {
	ctx := service.WithServiceStatusRequest(context.Background(), 0)
	fact := service.ServiceStatusAttemptObservation(ctx, "provider_capacity", time.Now())
	ops := service.NewOpsService(&serviceStatusPanicOpsRepository{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	batch := []opsErrorLogJob{{ops: ops, entry: &service.OpsInsertErrorLogInput{ServiceStatusObservation: fact}}, {ops: ops, entry: &service.OpsInsertErrorLogInput{ServiceStatusObservation: fact.Clone()}}}
	before := time.Now()
	flushOpsErrorLogBatch(batch)
	require.True(t, service.ServiceStatusSourceErrorSince(before), "真实批量 panic 恢复 owner 必须报告源缺口")
}

func TestServiceStatusSourceOpsWorkersDrainOwnedQueue(t *testing.T) {
	resetOpsErrorLoggerStateForTest(t)
	defer resetOpsErrorLoggerStateForTest(t)
	// 实际 worker 正在写出时关闭全局入口，仍须排空自己拥有的队列。
	repo := &serviceStatusDrainOpsRepository{started: make(chan struct{}, 1), release: make(chan struct{})}
	ops := service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	enqueueOpsErrorLog(ops, &service.OpsInsertErrorLogInput{ErrorType: "fixture"})
	<-repo.started
	done := make(chan bool, 1)
	go func() { done <- StopOpsErrorLogWorkers() }()
	close(repo.release)
	select {
	case drained := <-done:
		require.True(t, drained)
	case <-time.After(time.Second):
		t.Fatal("worker 停止后未排空自己的队列")
	}
}

type serviceStatusDrainOpsRepository struct {
	service.OpsRepository
	started chan struct{}
	release chan struct{}
}

func (r *serviceStatusDrainOpsRepository) InsertErrorLog(context.Context, *service.OpsInsertErrorLogInput) (int64, error) {
	r.started <- struct{}{}
	<-r.release
	return 1, nil
}
