package main

import (
	cfg "github.com/conductorone/baton-notion/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/config"
)

func main() {
	config.Generate("notion", cfg.Config)
}
