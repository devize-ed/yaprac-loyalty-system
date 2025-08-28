package accrual

// type storage interface {
// 	GetUnprocessedOrders(ctx context.Context) ([]*models.Order, error)
// 	UpdateOrder(ctx context.Context, order *models.Order) error
// }

// type AccrualService struct {
// 	accrualURL string
// 	storage    storage
// }

// func NewAccrualService(accrualURL string, storage storage) *AccrualService {
// 	return &AccrualService{
// 		accrualURL: accrualURL,
// 		storage:    storage,
// 	}
// }

// func (s *AccrualService) RunAccrual(ctx context.Context) error {
// 	// get all orders with status NEW or PROCESSING
// 	orders, err := s.storage.GetUnprocessedOrders(ctx)
// 	if err != nil {
// 		return fmt.Errorf("failed to get orders: %w", err)
// 	}

// 	for _, order := range orders {
// 		accrual, err := s.getAccrual(ctx, order.Number)
// 		if err != nil {
// 			return fmt.Errorf("failed to get accrual: %w", err)
// 		}
// 		order.Accrual = accrual
// 		err = s.storage.UpdateOrder(ctx, order)
// 		if err != nil {
// 			return fmt.Errorf("failed to update order: %w", err)
// 		}
// 	}
// }
