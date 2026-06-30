package usecase

import (
	"errors"

	"jk-api/internal/entity"
	"jk-api/internal/module/news/repository"
)

type NewsUsecase interface {
	CreateNews(req *CreateNewsRequest) (*entity.News, error)
	GetAllNews(page, limit int) ([]entity.News, int64, error)
	GetVisibleNews(roleName string, page, limit int) ([]entity.News, int64, error)
	GetNewsByID(id uint) (*entity.News, error)
	UpdateNews(id uint, req *UpdateNewsRequest) (*entity.News, error)
	DeleteNews(id uint) error
	UpdateImage(id uint, imagePath string) (*entity.News, error)
}

type CreateNewsRequest struct {
	Title     string `json:"title" validate:"required"`
	Body      string `json:"body" validate:"required"`
	Audience  string `json:"audience"`
	CreatedBy uint   `json:"-"`
}

type UpdateNewsRequest struct {
	Title    string  `json:"title"`
	Body     string  `json:"body"`
	Audience *string `json:"audience"`
}

type newsUsecase struct {
	newsRepo repository.NewsRepository
}

func NewNewsUsecase(newsRepo repository.NewsRepository) NewsUsecase {
	return &newsUsecase{newsRepo: newsRepo}
}

func validAudience(a string) bool {
	switch a {
	case "", "all", "customer", "staff":
		return true
	}
	return false
}

func (u *newsUsecase) CreateNews(req *CreateNewsRequest) (*entity.News, error) {
	if !validAudience(req.Audience) {
		return nil, errors.New("กลุ่มเป้าหมายไม่ถูกต้อง")
	}
	audience := req.Audience
	if audience == "" {
		audience = "all"
	}

	createdBy := req.CreatedBy
	news := &entity.News{
		Title:     req.Title,
		Body:      req.Body,
		Audience:  audience,
		CreatedBy: &createdBy,
	}
	if err := u.newsRepo.Create(news); err != nil {
		return nil, err
	}
	return news, nil
}

func (u *newsUsecase) GetAllNews(page, limit int) ([]entity.News, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	return u.newsRepo.FindAll(page, limit)
}

// GetVisibleNews returns the news visible to a given role on the home page:
// master sees everything; customer sees "all"/"customer"; owner/employee see
// "all"/"staff".
func (u *newsUsecase) GetVisibleNews(roleName string, page, limit int) ([]entity.News, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	var audiences []string
	switch roleName {
	case "master":
		audiences = nil
	case "customer":
		audiences = []string{"all", "customer"}
	default: // owner, employee
		audiences = []string{"all", "staff"}
	}
	return u.newsRepo.FindVisible(audiences, page, limit)
}

func (u *newsUsecase) GetNewsByID(id uint) (*entity.News, error) {
	return u.newsRepo.FindByID(id)
}

func (u *newsUsecase) UpdateNews(id uint, req *UpdateNewsRequest) (*entity.News, error) {
	news, err := u.newsRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("news not found")
	}

	if req.Title != "" {
		news.Title = req.Title
	}
	if req.Body != "" {
		news.Body = req.Body
	}
	if req.Audience != nil {
		if !validAudience(*req.Audience) {
			return nil, errors.New("กลุ่มเป้าหมายไม่ถูกต้อง")
		}
		audience := *req.Audience
		if audience == "" {
			audience = "all"
		}
		news.Audience = audience
	}

	if err := u.newsRepo.Update(news); err != nil {
		return nil, err
	}
	return news, nil
}

func (u *newsUsecase) DeleteNews(id uint) error {
	_, err := u.newsRepo.FindByID(id)
	if err != nil {
		return errors.New("news not found")
	}
	return u.newsRepo.Delete(id)
}

func (u *newsUsecase) UpdateImage(id uint, imagePath string) (*entity.News, error) {
	news, err := u.newsRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("news not found")
	}
	news.ImageURL = imagePath
	if err := u.newsRepo.Update(news); err != nil {
		return nil, err
	}
	return news, nil
}
