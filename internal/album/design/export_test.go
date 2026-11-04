package design

import "github.com/IliaSA101/sosninaalbums/internal/album/model"

// Внутренности design для внешних тестов (design_test).

func CaptionSource(p *model.Project, id string) *model.Caption { return captionSource(p, id) }
