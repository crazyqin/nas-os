// Package onboardingwizard 单元测试
package onboardingwizard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	return NewManager(logger)
}

func TestCreateSession(t *testing.T) {
	m := newTestManager(t)

	session, err := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeHome,
	})
	require.NoError(t, err)
	require.NotNil(t, session)

	assert.NotEmpty(t, session.ID)
	assert.Equal(t, TemplateTypeHome, session.TemplateType)
	assert.Len(t, session.Steps, 5)
	assert.False(t, session.IsCompleted)
	assert.NotNil(t, session.Progress)
	assert.Equal(t, 5, session.Progress.TotalSteps)
	assert.Equal(t, 0, session.Progress.CompletedSteps)
	assert.Equal(t, 0.0, session.Progress.Percentage)
}

func TestCreateSessionInvalidTemplate(t *testing.T) {
	m := newTestManager(t)

	_, err := m.CreateSession(CreateSessionRequest{
		TemplateType: "invalid",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCreateSessionEnterprise(t *testing.T) {
	m := newTestManager(t)

	session, err := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeEnterprise,
	})
	require.NoError(t, err)
	assert.Equal(t, TemplateTypeEnterprise, session.TemplateType)
}

func TestCreateSessionDeveloper(t *testing.T) {
	m := newTestManager(t)

	session, err := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeDeveloper,
	})
	require.NoError(t, err)
	assert.Equal(t, TemplateTypeDeveloper, session.TemplateType)
}

func TestGetSession(t *testing.T) {
	m := newTestManager(t)

	session, _ := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeHome,
	})

	got, err := m.GetSession(session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.ID, got.ID)
}

func TestGetSessionNotFound(t *testing.T) {
	m := newTestManager(t)

	_, err := m.GetSession("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestListSessions(t *testing.T) {
	m := newTestManager(t)

	m.CreateSession(CreateSessionRequest{TemplateType: TemplateTypeHome})
	m.CreateSession(CreateSessionRequest{TemplateType: TemplateTypeEnterprise})

	sessions := m.ListSessions()
	assert.Len(t, sessions, 2)
}

func TestCompleteStep(t *testing.T) {
	m := newTestManager(t)

	session, _ := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeHome,
	})

	stepID := session.Steps[0].ID
	config := NetworkConfig{
		Hostname:   "nas-home",
		DHCP:       true,
		DNSServers: []string{"8.8.8.8"},
	}

	updated, err := m.CompleteStep(session.ID, stepID, config)
	require.NoError(t, err)
	assert.Equal(t, StepStatusCompleted, updated.Steps[0].Status)
	assert.NotNil(t, updated.Steps[0].CompletedAt)
	assert.Equal(t, 20.0, updated.Progress.Percentage)
	assert.Equal(t, 1, updated.Progress.CompletedSteps)
}

func TestCompleteStepAlreadyCompleted(t *testing.T) {
	m := newTestManager(t)

	session, _ := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeHome,
	})

	stepID := session.Steps[0].ID
	m.CompleteStep(session.ID, stepID, nil)

	_, err := m.CompleteStep(session.ID, stepID, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already completed")
}

func TestCompleteStepSessionNotFound(t *testing.T) {
	m := newTestManager(t)

	_, err := m.CompleteStep("nonexistent", "step1", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCompleteStepStepNotFound(t *testing.T) {
	m := newTestManager(t)

	session, _ := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeHome,
	})

	_, err := m.CompleteStep(session.ID, "nonexistent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSkipStep(t *testing.T) {
	m := newTestManager(t)

	session, _ := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeHome,
	})

	// 找一个非必填步骤
	var optionalStep *Step
	for _, step := range session.Steps {
		if !step.Required {
			optionalStep = step
			break
		}
	}
	require.NotNil(t, optionalStep)

	updated, err := m.SkipStep(session.ID, optionalStep.ID)
	require.NoError(t, err)

	found := false
	for _, step := range updated.Steps {
		if step.ID == optionalStep.ID {
			assert.Equal(t, StepStatusSkipped, step.Status)
			assert.NotNil(t, step.SkippedAt)
			found = true
		}
	}
	assert.True(t, found)
	assert.Equal(t, 1, updated.Progress.SkippedSteps)
}

func TestSkipStepRequired(t *testing.T) {
	m := newTestManager(t)

	session, _ := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeHome,
	})

	// 找一个必填步骤
	var requiredStep *Step
	for _, step := range session.Steps {
		if step.Required {
			requiredStep = step
			break
		}
	}
	require.NotNil(t, requiredStep)

	_, err := m.SkipStep(session.ID, requiredStep.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestSkipStepAlreadySkipped(t *testing.T) {
	m := newTestManager(t)

	session, _ := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeHome,
	})

	var optionalStep *Step
	for _, step := range session.Steps {
		if !step.Required {
			optionalStep = step
			break
		}
	}

	m.SkipStep(session.ID, optionalStep.ID)
	_, err := m.SkipStep(session.ID, optionalStep.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already skipped")
}

func TestSkipStepAlreadyCompleted(t *testing.T) {
	m := newTestManager(t)

	session, _ := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeHome,
	})

	// 完成一个非必填步骤，然后尝试跳过
	var optionalStep *Step
	for _, step := range session.Steps {
		if !step.Required {
			optionalStep = step
			break
		}
	}

	m.CompleteStep(session.ID, optionalStep.ID, nil)
	_, err := m.SkipStep(session.ID, optionalStep.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already completed")
}

func TestUnskipStep(t *testing.T) {
	m := newTestManager(t)

	session, _ := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeHome,
	})

	var optionalStep *Step
	for _, step := range session.Steps {
		if !step.Required {
			optionalStep = step
			break
		}
	}

	m.SkipStep(session.ID, optionalStep.ID)
	updated, err := m.UnskipStep(session.ID, optionalStep.ID)
	require.NoError(t, err)

	found := false
	for _, step := range updated.Steps {
		if step.ID == optionalStep.ID {
			assert.Equal(t, StepStatusPending, step.Status)
			assert.Nil(t, step.SkippedAt)
			found = true
		}
	}
	assert.True(t, found)
}

func TestUnskipStepNotSkipped(t *testing.T) {
	m := newTestManager(t)

	session, _ := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeHome,
	})

	_, err := m.UnskipStep(session.ID, session.Steps[0].ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not skipped")
}

func TestGetProgress(t *testing.T) {
	m := newTestManager(t)

	session, _ := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeHome,
	})

	progress, err := m.GetProgress(session.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, progress.TotalSteps)
	assert.Equal(t, 0, progress.CompletedSteps)
	assert.Equal(t, 0.0, progress.Percentage)
	assert.False(t, progress.IsCompleted)
}

func TestCompleteAllSteps(t *testing.T) {
	m := newTestManager(t)

	session, _ := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeHome,
	})

	// 完成所有必填步骤
	for _, step := range session.Steps {
		if step.Required {
			_, err := m.CompleteStep(session.ID, step.ID, nil)
			require.NoError(t, err)
		}
	}

	// 跳过所有非必填步骤
	for _, step := range session.Steps {
		if !step.Required {
			_, err := m.SkipStep(session.ID, step.ID)
			require.NoError(t, err)
		}
	}

	updated, err := m.GetSession(session.ID)
	require.NoError(t, err)
	assert.True(t, updated.IsCompleted)
	assert.NotNil(t, updated.CompletedAt)
	assert.Equal(t, 100.0, updated.Progress.Percentage)
}

func TestGetTemplates(t *testing.T) {
	m := newTestManager(t)

	templates := m.GetTemplates()
	assert.Len(t, templates, 3)

	types := make(map[TemplateType]bool)
	for _, tmpl := range templates {
		types[tmpl.Type] = true
	}
	assert.True(t, types[TemplateTypeHome])
	assert.True(t, types[TemplateTypeEnterprise])
	assert.True(t, types[TemplateTypeDeveloper])
}

func TestGetRecommendations(t *testing.T) {
	m := newTestManager(t)

	tests := []struct {
		scenario string
		minApps  int
	}{
		{"home", 3},
		{"office", 3},
		{"development", 3},
		{"media", 3},
		{"backup", 2},
	}

	for _, tt := range tests {
		t.Run(tt.scenario, func(t *testing.T) {
			apps := m.GetRecommendations(tt.scenario)
			assert.GreaterOrEqual(t, len(apps), tt.minApps)

			for _, app := range apps {
				assert.NotEmpty(t, app.ID)
				assert.NotEmpty(t, app.Name)
				assert.NotEmpty(t, app.Description)
				assert.NotEmpty(t, app.Reason)
			}
		})
	}
}

func TestGetRecommendationsUnknown(t *testing.T) {
	m := newTestManager(t)

	apps := m.GetRecommendations("unknown")
	assert.Empty(t, apps)
}

func TestGetTemplate(t *testing.T) {
	tmpl := GetTemplate(TemplateTypeHome)
	require.NotNil(t, tmpl)
	assert.Equal(t, "家庭版", tmpl.Name)
	assert.Len(t, tmpl.Steps, 5)
	assert.NotEmpty(t, tmpl.Apps)

	tmpl = GetTemplate(TemplateTypeEnterprise)
	require.NotNil(t, tmpl)
	assert.Equal(t, "企业版", tmpl.Name)

	tmpl = GetTemplate(TemplateTypeDeveloper)
	require.NotNil(t, tmpl)
	assert.Equal(t, "开发者版", tmpl.Name)

	tmpl = GetTemplate("invalid")
	assert.Nil(t, tmpl)
}

func TestProgressPercentage(t *testing.T) {
	m := newTestManager(t)

	session, _ := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeHome,
	})

	// 完成第一步
	m.CompleteStep(session.ID, session.Steps[0].ID, nil)
	progress, _ := m.GetProgress(session.ID)
	assert.Equal(t, 20.0, progress.Percentage)

	// 完成第二步
	m.CompleteStep(session.ID, session.Steps[1].ID, nil)
	progress, _ = m.GetProgress(session.ID)
	assert.Equal(t, 40.0, progress.Percentage)
}

func TestSessionCustomData(t *testing.T) {
	m := newTestManager(t)

	session, _ := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeHome,
	})
	assert.NotNil(t, session.CustomData)
}

func TestStepOrder(t *testing.T) {
	m := newTestManager(t)

	session, _ := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeHome,
	})

	for i, step := range session.Steps {
		assert.Equal(t, i+1, step.Order, "step %d should have order %d", i, i+1)
	}
}

func TestStepTypes(t *testing.T) {
	m := newTestManager(t)

	session, _ := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeHome,
	})

	expectedTypes := []StepType{
		StepTypeNetwork,
		StepTypeStoragePool,
		StepTypeUserCreation,
		StepTypeRecommend,
		StepTypeAppInstall,
	}

	for i, step := range session.Steps {
		assert.Equal(t, expectedTypes[i], step.Type)
	}
}

func TestRequiredSteps(t *testing.T) {
	m := newTestManager(t)

	session, _ := m.CreateSession(CreateSessionRequest{
		TemplateType: TemplateTypeHome,
	})

	requiredCount := 0
	for _, step := range session.Steps {
		if step.Required {
			requiredCount++
		}
	}
	assert.Equal(t, 3, requiredCount)
}
