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

package dto

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// DeployedProductVersion is the version reference embedded in a deployed
// product — Label (not Name), ReleasedOn/EndOfLifeOn (not ReleasedDate/
// SupportEoLDate) to match the frontend's IdLabelRef shape
// (apps/customer-portal/webapp/src/types/common.ts), read directly off this
// field by DeploymentProductList.tsx.
type DeployedProductVersion struct {
	ID          string     `json:"id"`
	Label       string     `json:"label"`
	ReleasedOn  *time.Time `json:"releasedOn,omitempty"`
	EndOfLifeOn *time.Time `json:"endOfLifeOn,omitempty"`
}

// DeployedProductSummary is one item of the portal's response for
// POST /deployments/{deploymentId}/products/search — Deployment/Product as
// IDLabelRef (not Ref) and Cores/TPS as numbers (not strings) to match the
// frontend's own DeploymentProductItem type
// (apps/customer-portal/webapp/src/features/project-details/types/deployments.ts:
// cores?: number | null; tps?: number | null;). entity-service's own wire
// format sends Cores/TPS as ServiceNow strings (see DeployedProductView's
// doc comment in entity-service/internal/domain/entity.go) — parsed here
// rather than passed through, since the frontend's contract is this
// backend's to define, not entity-service's ServiceNow quirk to propagate.
type DeployedProductSummary struct {
	ID         string                  `json:"id"`
	Deployment *IDLabelRef             `json:"deployment,omitempty"`
	Product    *IDLabelRef             `json:"product,omitempty"`
	Version    *DeployedProductVersion `json:"version,omitempty"`
	Cores      *int                    `json:"cores,omitempty"`
	TPS        *float64                `json:"tps,omitempty"`
	Category   *string                 `json:"category,omitempty"`
	CreatedOn  time.Time               `json:"createdOn"`
	UpdatedOn  time.Time               `json:"updatedOn"`
}

// SearchDeployedProductsResponse is the portal's response for
// POST /deployments/{deploymentId}/products/search. TotalRecords (not
// Total) to match the frontend's shared pagination envelope.
type SearchDeployedProductsResponse struct {
	DeployedProducts []DeployedProductSummary `json:"deployedProducts"`
	TotalRecords     int                      `json:"totalRecords"`
	Limit            int                      `json:"limit"`
	Offset           int                      `json:"offset"`
	HasMore          bool                     `json:"hasMore"`
}

func mapDeployedProductVersion(v *entity.DeployedProductVersionRef) *DeployedProductVersion {
	if v == nil {
		return nil
	}
	return &DeployedProductVersion{
		ID:          v.ID,
		Label:       v.Name,
		ReleasedOn:  v.ReleasedDate,
		EndOfLifeOn: v.SupportEoLDate,
	}
}

// parseCores parses entity-service's ServiceNow-string Cores field into a
// number; an unparseable or absent value maps to nil rather than 0, so the
// frontend's `typeof item.cores === "number"` checks correctly treat it as
// unknown.
func parseCores(s *string) *int {
	if s == nil {
		return nil
	}
	n, err := strconv.Atoi(*s)
	if err != nil {
		return nil
	}
	return &n
}

// parseTPS parses entity-service's ServiceNow-string TPS field into a number.
func parseTPS(s *string) *float64 {
	if s == nil {
		return nil
	}
	n, err := strconv.ParseFloat(*s, 64)
	if err != nil {
		return nil
	}
	return &n
}

// MapSearchDeployedProducts builds the portal response from entity-service's SearchDeployedProductsResponse.
func MapSearchDeployedProducts(r entity.SearchDeployedProductsResponse) SearchDeployedProductsResponse {
	items := make([]DeployedProductSummary, 0, len(r.DeployedProducts))
	for _, d := range r.DeployedProducts {
		items = append(items, DeployedProductSummary{
			ID:         d.ID,
			Deployment: entityRefToIDLabel(&d.Deployment),
			Product:    entityRefToIDLabel(&d.Product),
			Version:    mapDeployedProductVersion(d.Version),
			Cores:      parseCores(d.Cores),
			TPS:        parseTPS(d.TPS),
			Category:   d.Category,
			CreatedOn:  d.CreatedOn,
			UpdatedOn:  d.UpdatedOn,
		})
	}
	return SearchDeployedProductsResponse{
		DeployedProducts: items,
		TotalRecords:     r.Total,
		Limit:            r.Limit,
		Offset:           r.Offset,
		HasMore:          r.HasMore,
	}
}

// ToSysID normalizes a dashed UUID or bare 32-hex string into a 32-character
// ServiceNow sysid (IdString) format by stripping hyphens.
func ToSysID(id string) string {
	return strings.ReplaceAll(id, "-", "")
}

// DeployedProductSearchFilters contains optional filters for searching deployed products.
type DeployedProductSearchFilters struct {
	ProductCategories []string `json:"productCategories,omitempty"`
}

// DeployedProductSearchRequest is the portal's request body for
// POST /deployments/{deploymentId}/products/search.
type DeployedProductSearchRequest struct {
	Pagination entity.Pagination             `json:"pagination"`
	Filters    *DeployedProductSearchFilters `json:"filters,omitempty"`
}

// BuildEntitySearchDeployedProductsRequest translates the portal's search
// request into entity-service's request shape, always scoping to the
// deployment in the URL (normalized to a canonical dashed UUID) — never a client-settable
// body field (same reasoning as BuildEntitySearchCasesRequest's projectID parameter).
func BuildEntitySearchDeployedProductsRequest(deploymentID string, req DeployedProductSearchRequest) entity.SearchDeployedProductsRequest {
	return entity.SearchDeployedProductsRequest{
		Pagination:    req.Pagination,
		DeploymentIDs: []string{toDashedID(deploymentID)},
	}
}

// DeployedProductCreateRequest is the portal's request body for
// POST /deployments/{deploymentId}/products, matching the frontend's
// PostDeploymentProductRequest type.
type DeployedProductCreateRequest struct {
	ProductID   string   `json:"productId"`
	VersionID   string   `json:"versionId"`
	ProjectID   string   `json:"projectId"`
	Cores       *int     `json:"cores,omitempty"`
	TPS         *float64 `json:"tps,omitempty"`
	Description *string  `json:"description,omitempty"`
}

// BuildEntityCreateDeployedProductRequest translates the portal's create
// request into entity-service's request shape, forcing DeploymentID from
// the path and normalizing all identifiers to sysids.
func BuildEntityCreateDeployedProductRequest(deploymentID string, req DeployedProductCreateRequest) entity.CreateDeployedProductRequest {
	return entity.CreateDeployedProductRequest{
		ProjectID:    ToSysID(req.ProjectID),
		DeploymentID: ToSysID(deploymentID),
		ProductID:    ToSysID(req.ProductID),
		VersionID:    ToSysID(req.VersionID),
		Cores:        req.Cores,
		TPS:          req.TPS,
		Description:  req.Description,
	}
}

// DeployedProductCreateResponse is the portal's response for
// POST /deployments/{deploymentId}/products.
type DeployedProductCreateResponse struct {
	ID        string    `json:"id"`
	CreatedOn time.Time `json:"createdOn"`
}

// MapDeployedProductCreate builds the portal response from entity-service's CreateDeployedProductResponse.
func MapDeployedProductCreate(r entity.CreateDeployedProductResponse) DeployedProductCreateResponse {
	return DeployedProductCreateResponse{
		ID:        r.DeployedProduct.ID,
		CreatedOn: r.DeployedProduct.CreatedOn,
	}
}

// DeployedProductUpdateRequest is the portal's request body for
// PATCH /deployments/{deploymentId}/products/{id}, matching the frontend's
// PatchDeploymentProductRequest type. Updates (a ProductUpdate[] list) is
// not exposed here — entity-service's UpdateDeployedProductRequest has no
// equivalent field, a genuine gap, not fixable in this dto layer alone.
type DeployedProductUpdateRequest struct {
	Cores       *int     `json:"cores,omitempty"`
	TPS         *float64 `json:"tps,omitempty"`
	Description *string  `json:"description,omitempty"`
	Active      *bool    `json:"active,omitempty"`
}

// BuildEntityUpdateDeployedProductRequest translates the portal's update
// request into entity-service's request shape, forcing DeploymentID from
// the path — entity-service documents this field as an IDOR-style scope
// guard ("the deployed product must belong to this deployment"), which the
// frontend's own request body never carries, so the path is the only
// reliable source (see PatchDeployment's doc comment on entityDeployment
// Client for the same reasoning).
func BuildEntityUpdateDeployedProductRequest(id, deploymentID string, req DeployedProductUpdateRequest) entity.UpdateDeployedProductRequest {
	depSysID := ToSysID(deploymentID)
	out := entity.UpdateDeployedProductRequest{
		ID:           ToSysID(id),
		DeploymentID: &depSysID,
		Cores:        req.Cores,
		TPS:          req.TPS,
		Active:       req.Active,
	}
	if req.Description != nil {
		if raw, err := json.Marshal(*req.Description); err == nil {
			out.Description = raw
		}
	}
	return out
}

// DeployedProductUpdateResponse is the portal's response for
// PATCH /deployments/{deploymentId}/products/{id}. Deliberately excludes
// entity-service's UpdatedBy (internal actor identity), consistent with the
// other update responses in this package.
type DeployedProductUpdateResponse struct {
	ID        string    `json:"id"`
	UpdatedOn time.Time `json:"updatedOn"`
}

// MapDeployedProductUpdate builds the portal response from entity-service's UpdateDeployedProductResponse.
func MapDeployedProductUpdate(r entity.UpdateDeployedProductResponse) DeployedProductUpdateResponse {
	return DeployedProductUpdateResponse{
		ID:        r.DeployedProduct.ID,
		UpdatedOn: r.DeployedProduct.UpdatedOn,
	}
}
