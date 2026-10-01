package core

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/e2engine/core/execute"
	runtimeconfig "github.com/e2engine/core/execute/runtime/config"
	"github.com/e2engine/core/mock"
	"github.com/e2engine/core/model"
	pkgerrors "github.com/e2engine/core/pkg/errors"
	"github.com/e2engine/core/repository"
)

func TestService_RunTest(t *testing.T) {
	t.Run("environment resolution fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		environmentRepo := mock.NewMockResourceRepository[repository.Environment](ctrl)

		environmentRepo.EXPECT().
			GetIDsByID(gomock.Any(), "missing-env").
			Return(nil, nil)

		environmentRepo.EXPECT().
			GetIDsByName(gomock.Any(), "missing-env").
			Return(nil, nil)

		service := newRunService(
			t,
			Repositories{
				Environment: environmentRepo,
			},
			&fakeLauncher{},
			10,
		)

		got, err := service.RunTest(
			context.Background(),
			TestRunRequest{
				EnvironmentRef: "missing-env",
				TestRef:        "test-001",
			},
		)

		if err == nil {
			t.Fatal("RunTest() error = nil, want error")
		}
		if got != "" {
			t.Errorf("RunTest() = %q, want empty execution ID", got)
		}
	})

	t.Run("test resolution fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		environmentRepo := mock.NewMockResourceRepository[repository.Environment](ctrl)
		testRepo := mock.NewMockTestRepository(ctrl)

		environmentRepo.EXPECT().
			GetIDsByID(gomock.Any(), "env").
			Return([]string{"environment-001"}, nil)

		environmentRepo.EXPECT().
			Get(gomock.Any(), "environment-001").
			Return(runRepositoryEnvironment(), nil)

		testRepo.EXPECT().
			GetIDsByID(gomock.Any(), "missing-test").
			Return(nil, nil)

		testRepo.EXPECT().
			GetIDsByName(gomock.Any(), "missing-test").
			Return(nil, nil)

		service := newRunService(
			t,
			Repositories{
				Environment: environmentRepo,
				Test:        testRepo,
			},
			&fakeLauncher{},
			10,
		)

		got, err := service.RunTest(
			context.Background(),
			TestRunRequest{
				EnvironmentRef: "env",
				TestRef:        "missing-test",
			},
		)

		if err == nil {
			t.Fatal("RunTest() error = nil, want error")
		}
		if got != "" {
			t.Errorf("RunTest() = %q, want empty execution ID", got)
		}
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		environmentRepo := mock.NewMockResourceRepository[repository.Environment](ctrl)
		testRepo := mock.NewMockTestRepository(ctrl)

		environmentRepo.EXPECT().
			GetIDsByID(gomock.Any(), "env").
			Return([]string{"environment-001"}, nil)

		environmentRepo.EXPECT().
			Get(gomock.Any(), "environment-001").
			Return(runRepositoryEnvironment(), nil)

		testRepo.EXPECT().
			GetIDsByID(gomock.Any(), "tes").
			Return([]string{"test-001"}, nil)

		testRepo.EXPECT().
			Get(gomock.Any(), "test-001").
			Return(runRepositoryTest("test-001", "health-check"), nil)

		launcher := &fakeLauncher{
			startTestResult: "execution-001",
		}

		service := newRunService(
			t,
			Repositories{
				Environment: environmentRepo,
				Test:        testRepo,
			},
			launcher,
			10,
		)

		got, err := service.RunTest(
			context.Background(),
			TestRunRequest{
				EnvironmentRef: "env",
				TestRef:        "tes",
			},
		)
		if err != nil {
			t.Fatalf("RunTest() error = %v", err)
		}

		if got != "execution-001" {
			t.Errorf(
				"RunTest() = %q, want %q",
				got,
				"execution-001",
			)
		}

		if launcher.startTestCalls != 1 {
			t.Fatalf(
				"StartTest calls = %d, want 1",
				launcher.startTestCalls,
			)
		}

		if launcher.testRequest.Environment.ID != "environment-001" {
			t.Errorf(
				"Environment.ID = %q, want %q",
				launcher.testRequest.Environment.ID,
				"environment-001",
			)
		}

		if launcher.testRequest.Test.ID != "test-001" {
			t.Errorf(
				"Test.ID = %q, want %q",
				launcher.testRequest.Test.ID,
				"test-001",
			)
		}
	})

	t.Run("launcher error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		environmentRepo := mock.NewMockResourceRepository[repository.Environment](ctrl)
		testRepo := mock.NewMockTestRepository(ctrl)

		environmentRepo.EXPECT().
			GetIDsByID(gomock.Any(), "env").
			Return([]string{"environment-001"}, nil)

		environmentRepo.EXPECT().
			Get(gomock.Any(), "environment-001").
			Return(runRepositoryEnvironment(), nil)

		testRepo.EXPECT().
			GetIDsByID(gomock.Any(), "tes").
			Return([]string{"test-001"}, nil)

		testRepo.EXPECT().
			Get(gomock.Any(), "test-001").
			Return(runRepositoryTest("test-001", "health-check"), nil)

		wantErr := errors.New("launcher failed")

		launcher := &fakeLauncher{
			startTestErr: wantErr,
		}

		service := newRunService(
			t,
			Repositories{
				Environment: environmentRepo,
				Test:        testRepo,
			},
			launcher,
			10,
		)

		got, err := service.RunTest(
			context.Background(),
			TestRunRequest{
				EnvironmentRef: "env",
				TestRef:        "tes",
			},
		)

		if !errors.Is(err, wantErr) {
			t.Fatalf(
				"RunTest() error = %v, want %v",
				err,
				wantErr,
			)
		}
		if got != "" {
			t.Errorf("RunTest() = %q, want empty execution ID", got)
		}
	})
}

func TestService_RunTestSuite(t *testing.T) {
	t.Run("no tests resolved", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		environmentRepo := mock.NewMockResourceRepository[repository.Environment](ctrl)
		testSuiteRepo := mock.NewMockResourceRepository[repository.TestSuite](ctrl)
		testRepo := mock.NewMockTestRepository(ctrl)

		environmentRepo.EXPECT().
			GetIDsByID(gomock.Any(), "env").
			Return([]string{"environment-001"}, nil)

		environmentRepo.EXPECT().
			Get(gomock.Any(), "environment-001").
			Return(runRepositoryEnvironment(), nil)

		testSuiteRepo.EXPECT().
			GetIDsByID(gomock.Any(), "suite").
			Return([]string{"testsuite-001"}, nil)

		testSuiteRepo.EXPECT().
			Get(gomock.Any(), "testsuite-001").
			Return(
				runRepositoryTestSuite(
					model.TestSelectorsSpec{
						Tags: []string{"smoke"},
					},
				),
				nil,
			)

		testRepo.EXPECT().
			GetByTag(gomock.Any(), "smoke").
			Return(nil, nil)

		launcher := &fakeLauncher{}

		service := newRunService(
			t,
			Repositories{
				Environment: environmentRepo,
				Test:        testRepo,
				TestSuite:   testSuiteRepo,
			},
			launcher,
			10,
		)

		got, err := service.RunTestSuite(
			context.Background(),
			TestSuiteRunRequest{
				EnvironmentRef: "env",
				TestSuiteRef:   "suite",
			},
		)

		if !errors.Is(err, pkgerrors.ErrNoTestsResolved) {
			t.Fatalf(
				"RunTestSuite() error = %v, want %v",
				err,
				pkgerrors.ErrNoTestsResolved,
			)
		}
		if got != "" {
			t.Errorf(
				"RunTestSuite() = %q, want empty execution ID",
				got,
			)
		}
		if launcher.startTestSuiteCalls != 0 {
			t.Errorf(
				"StartTestSuite calls = %d, want 0",
				launcher.startTestSuiteCalls,
			)
		}
	})

	t.Run("deduplicates and sorts resolved tests", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		environmentRepo := mock.NewMockResourceRepository[repository.Environment](ctrl)
		testSuiteRepo := mock.NewMockResourceRepository[repository.TestSuite](ctrl)
		testRepo := mock.NewMockTestRepository(ctrl)

		environmentRepo.EXPECT().
			GetIDsByID(gomock.Any(), "env").
			Return([]string{"environment-001"}, nil)

		environmentRepo.EXPECT().
			Get(gomock.Any(), "environment-001").
			Return(runRepositoryEnvironment(), nil)

		testSuiteRepo.EXPECT().
			GetIDsByID(gomock.Any(), "suite").
			Return([]string{"testsuite-001"}, nil)

		testSuiteRepo.EXPECT().
			Get(gomock.Any(), "testsuite-001").
			Return(
				runRepositoryTestSuite(
					model.TestSelectorsSpec{
						IDs:   []string{"tes-c"},
						Names: []string{"test-a"},
						Tags:  []string{"smoke"},
					},
				),
				nil,
			)

		testRepo.EXPECT().
			GetIDsByID(gomock.Any(), "tes-c").
			Return([]string{"test-c"}, nil)

		testRepo.EXPECT().
			Get(gomock.Any(), "test-c").
			Return(runRepositoryTest("test-c", "test-c"), nil)

		// Name resolution first checks ID prefixes.
		testRepo.EXPECT().
			GetIDsByID(gomock.Any(), "test-a").
			Return(nil, nil)

		testRepo.EXPECT().
			GetIDsByName(gomock.Any(), "test-a").
			Return([]string{"test-a"}, nil)

		testRepo.EXPECT().
			Get(gomock.Any(), "test-a").
			Return(runRepositoryTest("test-a", "test-a"), nil)

		// test-a is deliberately returned again by the tag lookup.
		testRepo.EXPECT().
			GetByTag(gomock.Any(), "smoke").
			Return([]repository.Test{
				*runRepositoryTest("test-b", "test-b"),
				*runRepositoryTest("test-a", "test-a"),
			}, nil)

		launcher := &fakeLauncher{
			startTestSuiteResult: "suite-execution-001",
		}

		service := newRunService(
			t,
			Repositories{
				Environment: environmentRepo,
				Test:        testRepo,
				TestSuite:   testSuiteRepo,
			},
			launcher,
			10,
		)

		got, err := service.RunTestSuite(
			context.Background(),
			TestSuiteRunRequest{
				EnvironmentRef: "env",
				TestSuiteRef:   "suite",
			},
		)
		if err != nil {
			t.Fatalf("RunTestSuite() error = %v", err)
		}

		if got != "suite-execution-001" {
			t.Errorf(
				"RunTestSuite() = %q, want %q",
				got,
				"suite-execution-001",
			)
		}

		if launcher.startTestSuiteCalls != 1 {
			t.Fatalf(
				"StartTestSuite calls = %d, want 1",
				launcher.startTestSuiteCalls,
			)
		}

		gotIDs := make([]string, len(launcher.testSuiteRequest.Tests))
		for i := range launcher.testSuiteRequest.Tests {
			gotIDs[i] = launcher.testSuiteRequest.Tests[i].ID
		}

		wantIDs := []string{
			"test-a",
			"test-b",
			"test-c",
		}

		if !reflect.DeepEqual(gotIDs, wantIDs) {
			t.Errorf(
				"resolved test IDs = %v, want %v",
				gotIDs,
				wantIDs,
			)
		}

		if launcher.testSuiteRequest.Environment.ID != "environment-001" {
			t.Errorf(
				"Environment.ID = %q, want %q",
				launcher.testSuiteRequest.Environment.ID,
				"environment-001",
			)
		}

		if launcher.testSuiteRequest.TestSuite.ID != "testsuite-001" {
			t.Errorf(
				"TestSuite.ID = %q, want %q",
				launcher.testSuiteRequest.TestSuite.ID,
				"testsuite-001",
			)
		}
	})

	t.Run("resolved tests limit exceeded", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		environmentRepo := mock.NewMockResourceRepository[repository.Environment](ctrl)
		testSuiteRepo := mock.NewMockResourceRepository[repository.TestSuite](ctrl)
		testRepo := mock.NewMockTestRepository(ctrl)

		environmentRepo.EXPECT().
			GetIDsByID(gomock.Any(), "env").
			Return([]string{"environment-001"}, nil)

		environmentRepo.EXPECT().
			Get(gomock.Any(), "environment-001").
			Return(runRepositoryEnvironment(), nil)

		testSuiteRepo.EXPECT().
			GetIDsByID(gomock.Any(), "suite").
			Return([]string{"testsuite-001"}, nil)

		testSuiteRepo.EXPECT().
			Get(gomock.Any(), "testsuite-001").
			Return(
				runRepositoryTestSuite(
					model.TestSelectorsSpec{
						Tags: []string{"smoke"},
					},
				),
				nil,
			)

		testRepo.EXPECT().
			GetByTag(gomock.Any(), "smoke").
			Return([]repository.Test{
				*runRepositoryTest("test-a", "test-a"),
				*runRepositoryTest("test-b", "test-b"),
				*runRepositoryTest("test-c", "test-c"),
			}, nil)

		launcher := &fakeLauncher{}

		service := newRunService(
			t,
			Repositories{
				Environment: environmentRepo,
				Test:        testRepo,
				TestSuite:   testSuiteRepo,
			},
			launcher,
			2,
		)

		got, err := service.RunTestSuite(
			context.Background(),
			TestSuiteRunRequest{
				EnvironmentRef: "env",
				TestSuiteRef:   "suite",
			},
		)

		if !errors.Is(err, pkgerrors.ErrExceededTestsLimit) {
			t.Fatalf(
				"RunTestSuite() error = %v, want %v",
				err,
				pkgerrors.ErrExceededTestsLimit,
			)
		}
		if got != "" {
			t.Errorf(
				"RunTestSuite() = %q, want empty execution ID",
				got,
			)
		}
		if launcher.startTestSuiteCalls != 0 {
			t.Errorf(
				"StartTestSuite calls = %d, want 0",
				launcher.startTestSuiteCalls,
			)
		}
	})

	t.Run("launcher error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		environmentRepo := mock.NewMockResourceRepository[repository.Environment](ctrl)
		testSuiteRepo := mock.NewMockResourceRepository[repository.TestSuite](ctrl)
		testRepo := mock.NewMockTestRepository(ctrl)

		environmentRepo.EXPECT().
			GetIDsByID(gomock.Any(), "env").
			Return([]string{"environment-001"}, nil)

		environmentRepo.EXPECT().
			Get(gomock.Any(), "environment-001").
			Return(runRepositoryEnvironment(), nil)

		testSuiteRepo.EXPECT().
			GetIDsByID(gomock.Any(), "suite").
			Return([]string{"testsuite-001"}, nil)

		testSuiteRepo.EXPECT().
			Get(gomock.Any(), "testsuite-001").
			Return(
				runRepositoryTestSuite(
					model.TestSelectorsSpec{
						Tags: []string{"smoke"},
					},
				),
				nil,
			)

		testRepo.EXPECT().
			GetByTag(gomock.Any(), "smoke").
			Return([]repository.Test{
				*runRepositoryTest("test-a", "test-a"),
			}, nil)

		wantErr := errors.New("launcher failed")

		launcher := &fakeLauncher{
			startTestSuiteErr: wantErr,
		}

		service := newRunService(
			t,
			Repositories{
				Environment: environmentRepo,
				Test:        testRepo,
				TestSuite:   testSuiteRepo,
			},
			launcher,
			10,
		)

		got, err := service.RunTestSuite(
			context.Background(),
			TestSuiteRunRequest{
				EnvironmentRef: "env",
				TestSuiteRef:   "suite",
			},
		)

		if !errors.Is(err, wantErr) {
			t.Fatalf(
				"RunTestSuite() error = %v, want %v",
				err,
				wantErr,
			)
		}
		if got != "" {
			t.Errorf(
				"RunTestSuite() = %q, want empty execution ID",
				got,
			)
		}
	})
}

type fakeLauncher struct {
	startTestCalls int
	testRequest    execute.TestRunRequest

	startTestResult string
	startTestErr    error

	startTestSuiteCalls int
	testSuiteRequest    execute.TestSuiteRunRequest

	startTestSuiteResult string
	startTestSuiteErr    error
}

func (l *fakeLauncher) StartTest(
	_ context.Context,
	req execute.TestRunRequest,
) (string, error) {
	l.startTestCalls++
	l.testRequest = req

	return l.startTestResult, l.startTestErr
}

func (l *fakeLauncher) StartTestSuite(
	_ context.Context,
	req execute.TestSuiteRunRequest,
) (string, error) {
	l.startTestSuiteCalls++
	l.testSuiteRequest = req

	return l.startTestSuiteResult, l.startTestSuiteErr
}

func newRunService(
	t *testing.T,
	repositories Repositories,
	launcher launcher,
	maxResolvedTests int,
) *Service {
	t.Helper()

	service, err := NewService(
		repositories,
		WithRuntimeConfig(&runtimeconfig.Config{
			MaxResolvedTests: maxResolvedTests,
		}),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	service.launcher = launcher

	return service
}

func runRepositoryEnvironment() *repository.Environment {
	spec, err := json.Marshal(validEnvironmentSpec())
	if err != nil {
		panic(err)
	}

	return &repository.Environment{
		ID:      "environment-001",
		Kind:    string(model.ResourceKindEnvironment),
		Name:    "local",
		Version: "1.0.0",
		Spec:    spec,
	}
}

func validEnvironmentSpec() model.EnvironmentSpec {
	return model.EnvironmentSpec{
		Services: []model.ServiceSpec{
			{
				ID:         "api",
				Kind:       model.ServiceKindHTTP,
				Mode:       model.ServiceModeReal,
				Address:    "api:8080",
				HTTPTarget: "http://localhost:8080",
			},
		},
	}
}

func runRepositoryTest(
	id string,
	name string,
) *repository.Test {
	spec, err := json.Marshal(validTestSpec())
	if err != nil {
		panic(err)
	}

	return &repository.Test{
		ID:      id,
		Kind:    string(model.ResourceKindTest),
		Name:    name,
		Version: "1.0.0",
		Spec:    spec,
	}
}

func validTestSpec() model.TestSpec {
	return model.TestSpec{
		Tags: []string{"smoke"},
		Request: model.RequestSpec{
			HTTP: &model.HTTPRequestSpec{
				Method: "GET",
				URL:    "http://localhost:8080/health",
			},
		},
		Expect: model.ExpectSpec{
			HTTP: &model.HTTPExpectSpec{
				Status: 200,
			},
		},
	}
}

func runRepositoryTestSuite(
	selectors model.TestSelectorsSpec,
) *repository.TestSuite {
	spec, err := json.Marshal(
		model.TestSuiteSpec{
			Selectors: selectors,
		},
	)
	if err != nil {
		panic(err)
	}

	return &repository.TestSuite{
		ID:      "testsuite-001",
		Kind:    string(model.ResourceKindTestSuite),
		Name:    "smoke-suite",
		Version: "1.0.0",
		Spec:    spec,
	}
}
