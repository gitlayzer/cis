package service

import (
	"context"
	"fmt"
	"cis/pkg/db"
	"cis/pkg/models"
)

type UserService interface {
	Register(ctx context.Context, username, password string) error
	Login(ctx context.Context, username, password string) (*models.User, error)
	GetUser(ctx context.Context, id uint) (*models.User, error)
}

type userServiceImpl struct{}

func NewUserService() UserService {
	return &userServiceImpl{}
}

func (s *userServiceImpl) Register(ctx context.Context, username, password string) error {
	user := &models.User{
		Username: username,
	}
	if err := user.SetPassword(password); err != nil {
		return err
	}

	if err := db.DB.Create(user).Error; err != nil {
		return fmt.Errorf("创建用户失败: %v", err)
	}
	return nil
}

func (s *userServiceImpl) Login(ctx context.Context, username, password string) (*models.User, error) {
	var user models.User
	if err := db.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, fmt.Errorf("用户不存在")
	}

	if !user.CheckPassword(password) {
		return nil, fmt.Errorf("密码错误")
	}

	return &user, nil
}

func (s *userServiceImpl) GetUser(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	if err := db.DB.First(&user, id).Error; err != nil {
		return nil, fmt.Errorf("用户不存在")
	}
	return &user, nil
} 