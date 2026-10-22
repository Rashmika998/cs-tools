// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// SLANotificationRuleService is the admin-facing surface of
// sla_notification_rule (R46-R51).
type SLANotificationRuleService interface {
	List(ctx context.Context) ([]domain.SLANotificationRule, error)
	Get(ctx context.Context, id string) (domain.SLANotificationRule, error)
	Create(ctx context.Context, req domain.UpsertSLANotificationRuleRequest) (domain.SLANotificationRule, error)
	Update(ctx context.Context, id string, req domain.UpsertSLANotificationRuleRequest) (domain.SLANotificationRule, error)
}

type slaNotificationRuleService struct {
	repo       repository.SLANotificationRuleRepository
	access     AccessService
	adminRoles []string
}

// NewSLANotificationRuleService constructs an SLANotificationRuleService.
// adminRoles is AUTH_ADMIN_ROLES.
func NewSLANotificationRuleService(repo repository.SLANotificationRuleRepository, access AccessService, adminRoles []string) SLANotificationRuleService {
	return &slaNotificationRuleService{repo: repo, access: access, adminRoles: adminRoles}
}

func (s *slaNotificationRuleService) List(ctx context.Context) ([]domain.SLANotificationRule, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return nil, err
	}
	return s.repo.List(ctx)
}

func (s *slaNotificationRuleService) Get(ctx context.Context, id string) (domain.SLANotificationRule, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return domain.SLANotificationRule{}, err
	}
	if id == "" {
		return domain.SLANotificationRule{}, &apierror.ValidationError{Msg: "notification rule id is required"}
	}
	return s.repo.Get(ctx, id)
}

func validateSLANotificationRuleRequest(req domain.UpsertSLANotificationRuleRequest) error {
	switch req.Channel {
	case "email":
	case "webhook":
		if req.WebhookURL == nil || *req.WebhookURL == "" {
			return &apierror.ValidationError{Msg: `webhookUrl is required when channel is "webhook"`}
		}
	default:
		return &apierror.ValidationError{Msg: `channel must be "email" or "webhook"`}
	}
	if req.ThresholdPercentage != nil && *req.ThresholdPercentage <= 0 {
		return &apierror.ValidationError{Msg: "thresholdPercentage must be greater than 0"}
	}
	if !req.Recipients.Assignee && !req.Recipients.Team && len(req.Recipients.Users) == 0 && len(req.Recipients.Groups) == 0 {
		return &apierror.ValidationError{Msg: "recipients must name at least one of assignee, team, users or groups"}
	}
	return nil
}

func (s *slaNotificationRuleService) Create(ctx context.Context, req domain.UpsertSLANotificationRuleRequest) (domain.SLANotificationRule, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return domain.SLANotificationRule{}, err
	}
	if err := validateSLANotificationRuleRequest(req); err != nil {
		return domain.SLANotificationRule{}, err
	}
	return s.repo.Create(ctx, req, slaActorEmail(ctx))
}

func (s *slaNotificationRuleService) Update(ctx context.Context, id string, req domain.UpsertSLANotificationRuleRequest) (domain.SLANotificationRule, error) {
	if err := requireSLAAdmin(ctx, s.access, s.adminRoles); err != nil {
		return domain.SLANotificationRule{}, err
	}
	if id == "" {
		return domain.SLANotificationRule{}, &apierror.ValidationError{Msg: "notification rule id is required"}
	}
	if err := validateSLANotificationRuleRequest(req); err != nil {
		return domain.SLANotificationRule{}, err
	}
	return s.repo.Update(ctx, id, req, slaActorEmail(ctx))
}
