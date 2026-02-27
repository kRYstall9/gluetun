package vpn

import (
	"context"
	"time"

	"github.com/qdm12/gluetun/internal/constants"
	"github.com/qdm12/gluetun/internal/models"
)

func ptrTo[T any](value T) *T { return &value }

// waitForError waits 100ms for an error in the waitError channel.
func (l *Loop) waitForError(ctx context.Context,
	waitError chan error,
) (err error) {
	const waitDurationForError = 100 * time.Millisecond
	timer := time.NewTimer(waitDurationForError)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
		return ctx.Err()
	case <-timer.C:
		return nil
	case err := <-waitError:
		close(waitError)
		if !timer.Stop() {
			<-timer.C
		}
		return err
	}
}

// crashed sets the health server error, signals the crashed status,
// and returns true if the VPN loop should retry, or false if it
// should stop retrying (when HEALTH_RESTART_VPN is off).
func (l *Loop) crashed(ctx context.Context, err error) (shouldRetry bool) {
	l.healthServer.SetError(err)
	l.signalOrSetStatus(constants.Crashed)
	if !*l.healthSettings.RestartVPN {
		l.logger.Error(err.Error())
		l.logger.Info("VPN will not be restarted because HEALTH_RESTART_VPN is set to off")
		return false
	}
	l.logAndWait(ctx, err)
	return true
}

func (l *Loop) signalOrSetStatus(status models.LoopStatus) {
	if l.userTrigger {
		l.userTrigger = false
		select {
		case l.running <- status:
		default: // receiver calling ApplyStatus dropped out
		}
	} else {
		l.statusManager.SetStatus(status)
	}
}

func (l *Loop) logAndWait(ctx context.Context, err error) {
	if err != nil {
		l.logger.Error(err.Error())
	}
	l.logger.Info("retrying in " + l.backoffTime.String())
	timer := time.NewTimer(l.backoffTime)
	l.backoffTime *= 2
	select {
	case <-timer.C:
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
	}
}
