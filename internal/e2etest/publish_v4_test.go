package e2etest

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type publishV4Suite struct {
	suite.Suite
}

func (s *publishV4Suite) TestPublish() {
	if testing.Short() {
		s.T().Skip("skipping testing in short mode")
	}

}

func (s *publishV4Suite) SetupSuite() {

}

func TestPublishV4Suite(t *testing.T) {
	suite.Run(t, new(publishV4Suite))
}
