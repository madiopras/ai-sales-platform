package health

type Service struct{}
type Status struct {
	Status string `json:"status"`
}

func NewService() *Service       { return &Service{} }
func (s *Service) Live() Status  { return Status{Status: "ok"} }
func (s *Service) Ready() Status { return Status{Status: "ok"} }
