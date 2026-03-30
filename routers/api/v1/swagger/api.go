package swagger

import api "forgejo.org/modules/structs"

// APIError
// swagger:response APIError
type swaggerAPIError struct {
	// in:body
	Body api.APIError `json:"body"`
}
