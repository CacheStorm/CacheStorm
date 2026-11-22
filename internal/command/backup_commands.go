package command

import (
	"errors"
	"strings"
	"sync/atomic"
)

// BackupController runs Redis 8.10's BACKUP for the server: a snapshot and
// the writes after it, kept in the backupdirname directory as a multi-part
// AOF until the backup is sealed.
type BackupController interface {
	Start() error
	Seal() error
	Abort() error
	Cleanup() error
	Status() BackupStatus
	// List returns the absolute paths of the files pinned so far: the
	// BASE file, the INCR file and the manifest, as each is finished.
	List() []string
}

// BackupStatus is what BACKUP STATUS reports.
type BackupStatus struct {
	State     string
	Error     string
	StartTime int64
	EndTime   int64
}

// backupSealedTTL is backup-sealed-ttl: the seconds after which a sealed
// backup is removed, 0 to keep it until BACKUP CLEANUP.
var backupSealedTTL atomic.Int64

// BackupSealedTTL returns backup-sealed-ttl.
func BackupSealedTTL() int64 { return backupSealedTTL.Load() }

// RegisterBackupCommands registers BACKUP.
func RegisterBackupCommands(router *Router) {
	router.Register(&CommandDef{Name: "BACKUP", Handler: cmdBACKUP})
}

const backupHelp = `BACKUP <subcommand> [<arg> [value] [opt] ...]. Subcommands are:
START
    Start a new backup into the configured 'backupdirname'.
SEAL
    Freeze the current backup (BASE + INCR + manifest).
ABORT
    Cancel a backup that has not been sealed yet.
CLEANUP
    Remove a sealed backup's files and return to idle.
STATUS
    Report the current backup state.
LIST
    List the immutable files pinned so far.
HELP
    Return this help.`

func cmdBACKUP(ctx *Context) error {
	if !checkRedisSubcommand(ctx, "backup", nil) {
		return nil
	}
	sub := strings.ToUpper(ctx.ArgString(0))
	if sub == "HELP" {
		return ctx.WriteArray(helpReply(backupHelp))
	}
	b := ctx.dispatcher().backupController()
	if b == nil {
		return ctx.WriteError(errors.New("ERR BACKUP is not available"))
	}
	var err error
	switch sub {
	case "START":
		err = b.Start()
	case "SEAL":
		err = b.Seal()
	case "ABORT":
		err = b.Abort()
	case "CLEANUP":
		err = b.Cleanup()
	case "STATUS":
		st := b.Status()
		w := ctx.Writer
		w.WriteMapHeader(4)
		w.WriteBulkStringItem("state")
		w.WriteBulkStringItem(st.State)
		w.WriteBulkStringItem("error")
		w.WriteBulkStringItem(st.Error)
		w.WriteBulkStringItem("start_time")
		w.WriteIntegerItem(st.StartTime)
		w.WriteBulkStringItem("end_time")
		w.WriteIntegerItem(st.EndTime)
		return w.Flush()
	case "LIST":
		files := b.List()
		w := ctx.Writer
		w.WriteArrayHeader(len(files))
		for _, f := range files {
			w.WriteBulkStringItem(f)
		}
		return w.Flush()
	}
	if err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}
