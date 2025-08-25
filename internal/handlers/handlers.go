package handlers

type Repository interface {
	CreateUser(user *models.User) error
	
}

type Handler struct {
	repository *repository.Repository
}

func NewHandler(r *repository.Repository) *Handler {
	return &Handler{repository: r}
}
