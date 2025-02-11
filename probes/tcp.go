// Package probes provides the logic to ping a target using
// a particular protocol
package probes

import (
	"net"
	"net/netip"
	"time"

	"github.com/pouriyajamshidi/tcping/v2/internal/utils"
	"github.com/pouriyajamshidi/tcping/v2/printers"
	"github.com/pouriyajamshidi/tcping/v2/types"
)

// handleConnError processes failed probes
func handleConnError(t *types.Tcping, startTime time.Time, elapsed time.Duration) {
	// if last probe had succeeded
	if !t.DestWasDown {
		t.StartOfDowntime = startTime
		uptimeDuration := t.StartOfDowntime.Sub(t.StartOfUptime)
		// set longest uptime since uptime is interrupted
		printers.SetLongestDuration(t.StartOfUptime, uptimeDuration, &t.LongestUptime)
		t.StartOfUptime = time.Time{}
		t.DestWasDown = true
	}

	t.TotalDowntime += elapsed
	t.LastUnsuccessfulProbe = startTime
	t.TotalUnsuccessfulProbes++
	t.OngoingUnsuccessfulProbes++

	t.PrintProbeFail(
		startTime,
		t.Options,
		t.OngoingUnsuccessfulProbes,
	)
}

// handleConnSuccess processes successful probes
func handleConnSuccess(t *types.Tcping, startTime time.Time, elapsed time.Duration, sourceAddr string, rtt float32) {
	if t.DestWasDown {
		t.StartOfUptime = startTime
		downtimeDuration := t.StartOfUptime.Sub(t.StartOfDowntime)
		// set longest downtime since downtime is interrupted
		printers.SetLongestDuration(t.StartOfDowntime, downtimeDuration, &t.LongestDowntime)
		t.PrintTotalDownTime(downtimeDuration)
		t.StartOfDowntime = time.Time{}
		t.DestWasDown = false
		t.OngoingUnsuccessfulProbes = 0
		t.OngoingSuccessfulProbes = 0
	}

	if t.StartOfUptime.IsZero() {
		t.StartOfUptime = startTime
	}

	t.TotalUptime += elapsed
	t.LastSuccessfulProbe = startTime
	t.TotalSuccessfulProbes++
	t.OngoingSuccessfulProbes++
	t.Rtt = append(t.Rtt, rtt)

	t.PrintProbeSuccess(
		startTime,
		sourceAddr,
		t.Options,
		t.OngoingSuccessfulProbes,
		rtt,
	)
}

// Ping checks target's availability using TCP
func Ping(tcping *types.Tcping) {
	var err error
	var conn net.Conn

	connStart := time.Now()

	if tcping.Options.NetworkInterface.Use {
		// The timeout value of this Dialer is set inside the `newNetworkInterface` function
		conn, err = tcping.Options.NetworkInterface.Dialer.Dial("tcp", tcping.Options.NetworkInterface.RemoteAddr.String())
	} else {
		ipAndPort := netip.AddrPortFrom(tcping.Options.IP, tcping.Options.Port)
		conn, err = net.DialTimeout("tcp", ipAndPort.String(), tcping.Options.Timeout)
	}

	connDuration := time.Since(connStart)
	elapsed := utils.MaxDuration(connDuration, tcping.Options.IntervalBetweenProbes)

	if err != nil {
		handleConnError(tcping, connStart, elapsed)
	} else {
		rtt := utils.NanoToMillisecond(connDuration.Nanoseconds())
		handleConnSuccess(tcping, connStart, elapsed, conn.LocalAddr().String(), rtt)

		conn.Close()
	}

	<-tcping.Ticker.C
}
