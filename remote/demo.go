package remote

import "encoding/json"

type Response struct {
	ErrorCode int         `json:"error_code"`
	ErrorNote string      `json:"error_note"`
	Data      json.RawMessage `json:"data"`
}
func (r Response) Scan(model interface{}) error {
	err := json.Unmarshal(r.Data, &model)
	if err != nil {
		return Error{
			Err:  ErrUnexpectedResponseData,
			Info: err.Error(),
		}
	}
	return nil
}
