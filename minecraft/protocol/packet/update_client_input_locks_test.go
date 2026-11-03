package packet

import (
	"bytes"
	"testing"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func TestInputLockCompatibilityPositionDoesNotChangeNativeWire(t *testing.T) {
	value := &UpdateClientInputLocks{Locks: ClientInputLockMovement, Position: mgl32.Vec3{123.5, 91.621, -55.5}}
	var output bytes.Buffer
	value.Marshal(protocol.NewWriter(&output, 0))
	if !bytes.Equal(output.Bytes(), []byte{4}) {
		t.Fatalf("native lock packet serialized the legacy position: %x", output.Bytes())
	}
	var decoded UpdateClientInputLocks
	decoded.Marshal(protocol.NewReader(&output, 0, true))
	if decoded.Locks != value.Locks || output.Len() != 0 {
		t.Fatalf("native lock flags changed: %+v, remaining=%d", decoded, output.Len())
	}
}
