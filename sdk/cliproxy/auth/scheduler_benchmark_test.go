package auth

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

type schedulerBenchmarkExecutor struct {
	id string
}

func (e schedulerBenchmarkExecutor) Identifier() string { return e.id }

func (e schedulerBenchmarkExecutor) Execute(ctx context.Context, auth *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, nil
}

func (e schedulerBenchmarkExecutor) ExecuteStream(ctx context.Context, auth *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	return nil, nil
}

func (e schedulerBenchmarkExecutor) Refresh(ctx context.Context, auth *Auth) (*Auth, error) {
	return auth, nil
}

func (e schedulerBenchmarkExecutor) CountTokens(ctx context.Context, auth *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, nil
}

func (e schedulerBenchmarkExecutor) HttpRequest(ctx context.Context, auth *Auth, req *http.Request) (*http.Response, error) {
	return nil, nil
}

func benchmarkManagerSetup(b *testing.B, total int, withPriority bool) (*Manager, string) {
	b.Helper()
	manager := NewManager(nil, &RoundRobinSelector{}, nil)
	const provider = "codex"
	manager.executor = schedulerBenchmarkExecutor{id: provider}

	reg := registry.GetGlobalRegistry()
	model := "bench-model"
	for index := 0; index < total; index++ {
		auth := &Auth{ID: fmt.Sprintf("bench-%s-%04d", provider, index), Provider: provider}
		if withPriority {
			priority := "0"
			if index%2 == 0 {
				priority = "10"
			}
			auth.Attributes = map[string]string{"priority": priority}
		}
		_, errRegister := manager.Register(context.Background(), auth)
		if errRegister != nil {
			b.Fatalf("Register(%s) error = %v", auth.ID, errRegister)
		}
		reg.RegisterClient(auth.ID, []*registry.ModelInfo{{ID: model}})
	}
	manager.syncScheduler()
	b.Cleanup(func() {
		for index := 0; index < total; index++ {
			reg.UnregisterClient(fmt.Sprintf("bench-%s-%04d", provider, index))
		}
	})

	return manager, model
}

func BenchmarkManagerPickNext500(b *testing.B) {
	manager, model := benchmarkManagerSetup(b, 500, false)
	ctx := context.Background()
	opts := cliproxyexecutor.Options{}
	tried := map[string]struct{}{}
	if _, _, errWarm := manager.pickNext(ctx, model, opts, tried); errWarm != nil {
		b.Fatalf("warmup pickNext error = %v", errWarm)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		auth, exec, errPick := manager.pickNext(ctx, model, opts, tried)
		if errPick != nil || auth == nil || exec == nil {
			b.Fatalf("pickNext failed: auth=%v exec=%v err=%v", auth, exec, errPick)
		}
	}
}

func BenchmarkManagerPickNext1000(b *testing.B) {
	manager, model := benchmarkManagerSetup(b, 1000, false)
	ctx := context.Background()
	opts := cliproxyexecutor.Options{}
	tried := map[string]struct{}{}
	if _, _, errWarm := manager.pickNext(ctx, model, opts, tried); errWarm != nil {
		b.Fatalf("warmup pickNext error = %v", errWarm)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		auth, exec, errPick := manager.pickNext(ctx, model, opts, tried)
		if errPick != nil || auth == nil || exec == nil {
			b.Fatalf("pickNext failed: auth=%v exec=%v err=%v", auth, exec, errPick)
		}
	}
}

func BenchmarkManagerPickNextPriority500(b *testing.B) {
	manager, model := benchmarkManagerSetup(b, 500, true)
	ctx := context.Background()
	opts := cliproxyexecutor.Options{}
	tried := map[string]struct{}{}
	if _, _, errWarm := manager.pickNext(ctx, model, opts, tried); errWarm != nil {
		b.Fatalf("warmup pickNext error = %v", errWarm)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		auth, exec, errPick := manager.pickNext(ctx, model, opts, tried)
		if errPick != nil || auth == nil || exec == nil {
			b.Fatalf("pickNext failed: auth=%v exec=%v err=%v", auth, exec, errPick)
		}
	}
}

func BenchmarkManagerPickNextPriority1000(b *testing.B) {
	manager, model := benchmarkManagerSetup(b, 1000, true)
	ctx := context.Background()
	opts := cliproxyexecutor.Options{}
	tried := map[string]struct{}{}
	if _, _, errWarm := manager.pickNext(ctx, model, opts, tried); errWarm != nil {
		b.Fatalf("warmup pickNext error = %v", errWarm)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		auth, exec, errPick := manager.pickNext(ctx, model, opts, tried)
		if errPick != nil || auth == nil || exec == nil {
			b.Fatalf("pickNext failed: auth=%v exec=%v err=%v", auth, exec, errPick)
		}
	}
}

func BenchmarkManagerPickNextAndMarkResult1000(b *testing.B) {
	manager, model := benchmarkManagerSetup(b, 1000, false)
	ctx := context.Background()
	opts := cliproxyexecutor.Options{}
	tried := map[string]struct{}{}
	if _, _, errWarm := manager.pickNext(ctx, model, opts, tried); errWarm != nil {
		b.Fatalf("warmup pickNext error = %v", errWarm)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		auth, _, errPick := manager.pickNext(ctx, model, opts, tried)
		if errPick != nil || auth == nil {
			b.Fatalf("pickNext failed: auth=%v err=%v", auth, errPick)
		}
		manager.MarkResult(ctx, Result{AuthID: auth.ID, Model: model, Success: true})
	}
}
