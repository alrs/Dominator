package rpcd

import (
	"runtime"

	"github.com/Symantec/Dominator/lib/srpc"
	"github.com/Symantec/Dominator/objectserver/rpcd/lib"
)

func (t *srpcType) AddObjects(conn *srpc.Conn) error {
	defer runtime.GC() // An opportune time to take out the garbage.
	if t.replicationMaster == "" {
		return lib.AddObjects(conn, conn, conn, t.objectServer, t.logger)
	}
	return lib.AddObjectsWithMaster(conn, conn, conn, t.objectServer,
		t.replicationMaster, t.logger)
}
