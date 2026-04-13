package server

import (
	"context"
	"fmt"
)

// Install pulls the image and runs initial provisioning steps.
func (s *Server) Install(ctx context.Context) error {
	s.ForceState(StateInstalling)
	if err := s.docker.PullImage(ctx, s.Image); err != nil {
		s.ForceState(StateError)
		return fmt.Errorf("pulling install image: %w", err)
	}
	s.ForceState(StateStopped)
	s.console.Append("installation complete")
	return nil
}
