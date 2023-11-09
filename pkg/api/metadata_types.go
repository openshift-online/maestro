/*
Copyright (c) 2018 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This file contains the API metadata types used by the maestro.

package api

import (
	"time"

	"gorm.io/gorm"
)

// CollectionMetadata represents a collection.
type CollectionMetadata struct {
	ID   string `json:"id"`
	HREF string `json:"href"`
	Kind string `json:"kind"`
}

// VersionMetadata represents a version.
type VersionMetadata struct {
	ID          string               `json:"id"`
	HREF        string               `json:"href"`
	Kind        string               `json:"kind"`
	Collections []CollectionMetadata `json:"collections"`
}

// Metadata api metadata.
type Metadata struct {
	ID       string            `json:"id"`
	HREF     string            `json:"href"`
	Kind     string            `json:"kind"`
	Versions []VersionMetadata `json:"versions"`
}

// Meta is base model definition, embedded in all kinds
type Meta struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// List Paging metadata
type PagingMeta struct {
	Page  int
	Size  int64
	Total int64
}
