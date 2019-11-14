package reflection

import (
	"testing"

	"github.com/lbryio/reflector.go/peer"

	"github.com/sirupsen/logrus"
)

func TestGetBlobs(t *testing.T) {
	c := peer.Client{}
	err := c.Connect("reflector.lbry.com:5567")
	if err != nil {
		logrus.Error(err)
	}

	stream, err := c.GetBlob("3dbeeaaeba62b2dce2f796d736e40d9c0182ad3122230abcd0787c4e98d5ae01948c19149dd00aacc19f72834625085b")
	if err != nil {
		logrus.Error(err)
	}

	logrus.Info(stream.Size())

}
