// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package types

import "forgejo.org/modules/translation"

type OwnerType string

const (
	OwnerTypeSystemGlobal OwnerType = "system-global"
	OwnerTypeIndividual   OwnerType = "individual"
	OwnerTypeRepository   OwnerType = "repository"
	OwnerTypeOrganization OwnerType = "organization"
	OwnerTypeStackit      OwnerType = "stackit"
)

// >>> @@@ STACKIT CODE @@@
var AllOwnerTypes = []OwnerType{
	OwnerTypeSystemGlobal,
	OwnerTypeIndividual,
	OwnerTypeRepository,
	OwnerTypeOrganization,
	OwnerTypeStackit,
}

// >>> @@@ STACKIT CODE @@@

func (o OwnerType) LocaleString(locale translation.Locale) string {
	switch o {
	case OwnerTypeSystemGlobal:
		return locale.TrString("concept_system_global")
	case OwnerTypeIndividual:
		return locale.TrString("concept_user_individual")
	case OwnerTypeRepository:
		return locale.TrString("concept_code_repository")
	case OwnerTypeOrganization:
		return locale.TrString("concept_user_organization")
	case OwnerTypeStackit:
		return locale.TrString("concept_system_stackit")
	}
	return locale.TrString("unknown")
}
