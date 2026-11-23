//go:build windows

// Sicherheitspaket: steuert den in Windows eingebauten Microsoft Defender und
// holt bei Bedarf eine Zweitmeinung vom Microsoft Safety Scanner.
package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"
)

// psSecurity wird an das Prelude angehängt (Funktion Get-NrmmSecurity).
const psSecurity = `
function Get-NrmmAge($v) { if ($null -eq $v) { return -1 }; $n = [long]$v; if ($n -gt 36500) { return -1 }; return $n }
function Get-NrmmTime($v) { if ($v -and $v.Year -gt 1601) { return $v.ToString('yyyy-MM-ddTHH:mm:ss') }; return $null }
function Get-NrmmSecurity {
  $s = $null; $p = $null; $err = $null
  try { $s = Get-MpComputerStatus -ErrorAction Stop } catch { $err = $_.Exception.Message }
  try { $p = Get-MpPreference -ErrorAction Stop } catch {}
  $byId = @{}
  try { Get-MpThreat -ErrorAction Stop | ForEach-Object { $byId[[string]$_.ThreatID] = $_ } } catch {}
  $det = @()
  try {
    $det = @(Get-MpThreatDetection -ErrorAction Stop | Sort-Object InitialDetectionTime -Descending | Select-Object -First 40 | ForEach-Object {
      $t = $byId[[string]$_.ThreatID]
      [pscustomobject]@{
        threatId = [string]$_.ThreatID
        name = $(if ($t) { [string]$t.ThreatName } else { '' })
        severity = $(if ($t) { [int]$t.SeverityID } else { 0 })
        active = $(if ($t) { [bool]$t.IsActive } else { $false })
        statusId = [int]$_.ThreatStatusID
        time = Get-NrmmTime $_.InitialDetectionTime
        resources = @($_.Resources | Select-Object -First 5 | ForEach-Object { ([string]$_) -replace '^[A-Za-z]+:_', '' })
        user = [string]$_.DomainUser
        process = [string]$_.ProcessName
      }
    })
  } catch {}
  $av = @()
  try {
    $av = @(Get-CimInstance -Namespace root/SecurityCenter2 -ClassName AntiVirusProduct -ErrorAction Stop | ForEach-Object {
      $h = '{0:x6}' -f [int]$_.productState
      [pscustomobject]@{ name = [string]$_.displayName; enabled = ($h.Substring(2,2) -in '10','11'); upToDate = ($h.Substring(4,2) -eq '00') }
    })
  } catch {}
  $o = [pscustomobject]@{
    available = [bool]$s; error = $err
    mode = [string]$s.AMRunningMode
    enabled = [bool]$s.AntivirusEnabled; realtime = [bool]$s.RealTimeProtectionEnabled
    behavior = [bool]$s.BehaviorMonitorEnabled; tamper = [bool]$s.IsTamperProtected
    signatureVersion = [string]$s.AntivirusSignatureVersion; signatureUpdated = Get-NrmmTime $s.AntivirusSignatureLastUpdated
    signatureAgeDays = Get-NrmmAge $s.AntivirusSignatureAge
    engineVersion = [string]$s.AMEngineVersion; productVersion = [string]$s.AMProductVersion
    quickScanEnd = Get-NrmmTime $s.QuickScanEndTime; quickScanAgeDays = Get-NrmmAge $s.QuickScanAge
    fullScanEnd = Get-NrmmTime $s.FullScanEndTime; fullScanAgeDays = Get-NrmmAge $s.FullScanAge
    computerState = [int]$s.ComputerState
    pua = $(if ($p) { [int]$p.PUAProtection } else { -1 })
    cloud = $(if ($p) { [int]$p.MAPSReporting } else { -1 })
    network = $(if ($p) { [int]$p.EnableNetworkProtection } else { -1 })
    ransomware = $(if ($p) { [int]$p.EnableControlledFolderAccess } else { -1 })
    activeThreats = @($byId.Values | Where-Object { $_.IsActive }).Count
    threats = $det; products = $av
  }
  ConvertTo-Json -InputObject $o -Depth 5 -Compress
}
function Get-NrmmActiveCount { try { return @(Get-MpThreat -ErrorAction Stop | Where-Object { $_.IsActive }).Count } catch { return 0 } }
`

var psSecurityScripts = map[string]string{
	"security.status": `Write-NrmmData (Get-NrmmSecurity)`,
	"security.scan": `
if ($env:NRMM_SCANTYPE -eq 'full') { Start-MpScan -ScanType FullScan; 'Vollständiger Scan abgeschlossen.' } else { Start-MpScan -ScanType QuickScan; 'Schnellscan abgeschlossen.' }
$n = Get-NrmmActiveCount
if ($n -gt 0) { "Achtung: $n aktive Bedrohung(en) gefunden. Mit „Bedrohungen entfernen“ bereinigen." } else { 'Keine aktiven Bedrohungen gefunden.' }
Write-NrmmData (Get-NrmmSecurity)
`,
	"security.update": `
try { Update-MpSignature -ErrorAction Stop } catch { Update-MpSignature -UpdateSource MicrosoftUpdateServer -ErrorAction Stop }
$s = Get-MpComputerStatus
"Virensignaturen sind aktuell (Version $($s.AntivirusSignatureVersion))."
Write-NrmmData (Get-NrmmSecurity)
`,
	"security.removeThreats": `
$before = Get-NrmmActiveCount
if ($before -gt 0) { Remove-MpThreat -ErrorAction Stop }
Start-Sleep -Seconds 3
$after = Get-NrmmActiveCount
if ($before -eq 0) { 'Es gab keine aktiven Bedrohungen.' } elseif ($after -eq 0) { "$before Bedrohung(en) entfernt." } else { "Noch $after von $before Bedrohung(en) aktiv. Ein Neustart oder ein Offline-Scan kann nötig sein." }
Write-NrmmData (Get-NrmmSecurity)
`,
	"security.harden": `
$ok = New-Object System.Collections.ArrayList; $bad = New-Object System.Collections.ArrayList
function Set-Nrmm($label, [scriptblock]$sb) { try { & $sb; [void]$ok.Add($label) } catch { [void]$bad.Add("$label – $($_.Exception.Message)") } }
$cur = Get-MpPreference
if ($cur.DisableRealtimeMonitoring) { Set-Nrmm 'Echtzeitschutz eingeschaltet' { Set-MpPreference -DisableRealtimeMonitoring $false } }
if ($env:NRMM_PUA -eq '1') { Set-Nrmm 'Schutz vor unerwünschten Programmen (PUA): an' { Set-MpPreference -PUAProtection Enabled } } else { Set-Nrmm 'Schutz vor unerwünschten Programmen (PUA): aus' { Set-MpPreference -PUAProtection Disabled } }
if ($env:NRMM_CLOUD -eq '1') { Set-Nrmm 'Cloud-Schutz: an' { Set-MpPreference -MAPSReporting Advanced -SubmitSamplesConsent SendSafeSamples } } else { Set-Nrmm 'Cloud-Schutz: aus' { Set-MpPreference -MAPSReporting Disabled } }
if ($env:NRMM_NETWORK -eq '1') { Set-Nrmm 'Netzwerkschutz (gefährliche Webseiten): an' { Set-MpPreference -EnableNetworkProtection Enabled } } else { Set-Nrmm 'Netzwerkschutz: aus' { Set-MpPreference -EnableNetworkProtection Disabled } }
if ($env:NRMM_RANSOMWARE -eq '1') { Set-Nrmm 'Ransomware-Schutz (überwachter Ordnerzugriff): an' { Set-MpPreference -EnableControlledFolderAccess Enabled } } else { Set-Nrmm 'Ransomware-Schutz: aus' { Set-MpPreference -EnableControlledFolderAccess Disabled } }
$ok | ForEach-Object { "✓ $_" }
$bad | ForEach-Object { "✗ $_" }
if ($bad.Count -gt 0) { 'Hinweis: Ist der Manipulationsschutz aktiv oder ein anderer Virenscanner installiert, lassen sich manche Einstellungen nur direkt am PC ändern.' }
Write-NrmmData (Get-NrmmSecurity)
if ($ok.Count -eq 0) { exit 1 }
`,
	"security.pause": `
$min = [int]$env:NRMM_MINUTES
if (-not (Get-MpComputerStatus).RealTimeProtectionEnabled) {
  "Der Echtzeitschutz war bereits aus. Er wird in $min Minuten automatisch eingeschaltet."
} else {
  try { Set-MpPreference -DisableRealtimeMonitoring $true -ErrorAction Stop } catch {}
  Start-Sleep -Seconds 2
  if ((Get-MpComputerStatus).RealTimeProtectionEnabled) {
    $tp = (Get-MpComputerStatus).IsTamperProtected
    if ($tp) {
      throw 'Windows hat das Ausschalten blockiert, weil der Manipulationsschutz aktiv ist – das ist ein Schutz gegen Schädlinge und lässt sich aus der Ferne absichtlich nicht umgehen. So geht es trotzdem: Bildschirm verbinden, dann im Menü „Windows-Sicherheit am PC öffnen“ und dort Manipulationsschutz und Echtzeitschutz kurz ausschalten. Windows schaltet den Echtzeitschutz nach einiger Zeit von selbst wieder ein; den Manipulationsschutz danach bitte wieder einschalten.'
    }
    throw 'Der Echtzeitschutz ließ sich nicht ausschalten (evtl. durch eine Richtlinie oder einen anderen Virenscanner gesteuert).'
  }
  "Echtzeitschutz für $min Minuten pausiert. Danach schaltet er sich automatisch wieder ein."
}
Write-NrmmData (Get-NrmmSecurity)
`,
	"security.resume": `
try { Set-MpPreference -DisableRealtimeMonitoring $false -ErrorAction Stop } catch {}
Start-Sleep -Seconds 2
if (-not (Get-MpComputerStatus).RealTimeProtectionEnabled) { throw 'Der Echtzeitschutz ist noch aus. Bitte am PC unter Windows-Sicherheit prüfen.' }
'Der Echtzeitschutz ist wieder an.'
Write-NrmmData (Get-NrmmSecurity)
`,
	"security.offlineScan": `
Start-MpWDOScan
'Der PC startet gleich neu und prüft sich vor dem Windows-Start (dauert ca. 15 Minuten).'
`,
}

func execSecurity(ctx context.Context, cmd Cmd, a map[string]any) Result {
	if cmd.Type == "security.msert" {
		return runMsert(ctx, argString(a, "type"))
	}
	script, ok := psSecurityScripts[cmd.Type]
	if !ok {
		return Result{OK: false, Output: "Unbekannte Aktion: " + cmd.Type}
	}
	env := map[string]string{
		"NRMM_MINUTES":  fmt.Sprint(argInt(a, "minutes")),
		"NRMM_SCANTYPE": argString(a, "type"),
		"NRMM_PUA":      boolEnv(argBool(a, "pua")), "NRMM_CLOUD": boolEnv(argBool(a, "cloud")),
		"NRMM_NETWORK": boolEnv(argBool(a, "network")), "NRMM_RANSOMWARE": boolEnv(argBool(a, "ransomware")),
	}
	out, data, err := runPS(ctx, psSecurity+script, env, true)
	if err != nil {
		if out == "" {
			out = err.Error()
		}
		return Result{OK: false, Output: cleanPSError(out), Data: data}
	}
	switch cmd.Type {
	case "security.pause":
		until := time.Now().Add(time.Duration(max(argInt(a, "minutes"), 5)) * time.Minute)
		if err := armPause(until); err != nil {
			// Ohne sichere Rückkehr kein Pausieren: sofort wieder einschalten.
			runPS(context.Background(), psSecurity+psSecurityScripts["security.resume"], nil, true)
			return Result{OK: false, Output: "Automatisches Wiedereinschalten konnte nicht eingerichtet werden – Schutz bleibt an. (" + err.Error() + ")"}
		}
		out += "\nWieder an um " + until.Format("15:04") + " Uhr."
	case "security.resume":
		disarmPause()
	}
	return Result{OK: true, Output: out, Data: data}
}

// ---------- Schutz-Pause mit doppelter Absicherung ----------
// 1. Der Agent prüft alle 20 Sekunden die Datei pause.json.
// 2. Eine geplante Aufgabe schaltet den Schutz zur Endzeit ein – auch wenn der Agent nicht läuft.

const taskResume = `NicolajRMM\Schutz-wieder-an`

func pauseFile() string { return filepath.Join(dataDir(), "pause.json") }

func armPause(until time.Time) error {
	if err := os.WriteFile(pauseFile(), []byte(until.UTC().Format(time.RFC3339)), 0o600); err != nil {
		return err
	}
	return createTask(taskResume, resumeTaskXML(until))
}

func disarmPause() {
	os.Remove(pauseFile())
	quiet(sys32("schtasks.exe"), "/Delete", "/TN", taskResume, "/F")
}

func resumeTaskXML(until time.Time) string {
	ps := sys32(`WindowsPowerShell\v1.0\powershell.exe`)
	return `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo><Author>Nicolaj Webdesign</Author><Description>Nicolaj Fernwartung – schaltet den Echtzeitschutz nach einer Pause wieder ein</Description></RegistrationInfo>
  <Triggers><TimeTrigger><StartBoundary>` + until.Format("2006-01-02T15:04:05") + `</StartBoundary><Enabled>true</Enabled></TimeTrigger><BootTrigger><Enabled>true</Enabled><Delay>PT2M</Delay></BootTrigger></Triggers>
  <Principals><Principal id="Author"><UserId>S-1-5-18</UserId><RunLevel>HighestAvailable</RunLevel></Principal></Principals>
  <Settings><MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy><DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries><StopIfGoingOnBatteries>false</StopIfGoingOnBatteries><StartWhenAvailable>true</StartWhenAvailable><ExecutionTimeLimit>PT10M</ExecutionTimeLimit><Enabled>true</Enabled><Hidden>false</Hidden></Settings>
  <Actions Context="Author"><Exec><Command>"` + xmlEscape(ps) + `"</Command><Arguments>-NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "if (-not (Test-Path '` + xmlEscape(pauseFile()) + `') -or ((Get-Date) -ge [datetime]::Parse((Get-Content '` + xmlEscape(pauseFile()) + `')))) { Set-MpPreference -DisableRealtimeMonitoring $false; Remove-Item '` + xmlEscape(pauseFile()) + `' -ErrorAction SilentlyContinue }"</Arguments></Exec></Actions>
</Task>`
}

// pauseWatcher schaltet den Schutz wieder ein, sobald die Pause abgelaufen ist (auch nach einem Neustart).
func pauseWatcher(c *Client) {
	for {
		if b, err := os.ReadFile(pauseFile()); err == nil {
			until, perr := time.Parse(time.RFC3339, strings.TrimSpace(string(b)))
			if perr != nil || time.Now().After(until) {
				logf("Schutz-Pause abgelaufen – schalte Echtzeitschutz wieder ein")
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				_, _, err := runPS(ctx, psSecurity+psSecurityScripts["security.resume"], nil, true)
				cancel()
				if err != nil {
					logf("Wiedereinschalten fehlgeschlagen: %v", err)
				}
				disarmPause()
				if inv := collectInventory(); len(inv) > 0 {
					c.postRaw("/api/agent/inventory", inv)
				}
			}
		}
		time.Sleep(20 * time.Second)
	}
}

func openSecuritySettings() error {
	return exec.Command(filepath.Join(os.Getenv("SystemRoot"), "explorer.exe"), "windowsdefender://threatsettings").Start()
}

// runMsert lädt den Microsoft Safety Scanner frisch von Microsoft, prüft die Signatur
// und lässt ihn als zweite Meinung laufen.
func runMsert(ctx context.Context, typ string) Result {
	const url = "https://go.microsoft.com/fwlink/?LinkId=212732" // 64-Bit, offizieller Microsoft-Link
	dir := filepath.Join(dataDir(), "tmp")
	os.MkdirAll(dir, 0o700)
	exe := filepath.Join(dir, "msert.exe")
	defer os.Remove(exe)

	var log strings.Builder
	log.WriteString("Lade Microsoft Safety Scanner …\n")
	if err := download(ctx, url, exe); err != nil {
		return Result{OK: false, Output: log.String() + "Download fehlgeschlagen: " + err.Error()}
	}
	check := `$s = Get-AuthenticodeSignature -FilePath $env:NRMM_FILE
if ($s.Status -ne 'Valid' -or $s.SignerCertificate.Subject -notmatch 'O=Microsoft Corporation') { throw "Signatur ungültig ($($s.Status)): $($s.SignerCertificate.Subject)" }
'Signatur von Microsoft geprüft.'`
	out, _, err := runPS(ctx, check, map[string]string{"NRMM_FILE": exe}, true)
	log.WriteString(out + "\n")
	if err != nil {
		return Result{OK: false, Output: log.String() + "Aus Sicherheitsgründen nicht ausgeführt."}
	}

	logPath := filepath.Join(os.Getenv("SystemRoot"), "debug", "msert.log")
	var before int64
	if st, err := os.Stat(logPath); err == nil {
		before = st.Size()
	}
	args := []string{"/Q", "/N"} // schnell, nur melden
	if typ == "full" {
		args = []string{"/Q", "/F:Y"} // gründlich, Funde automatisch entfernen
	}
	log.WriteString(map[bool]string{true: "Gründlicher Scan läuft (kann über eine Stunde dauern) …\n", false: "Schneller Scan läuft …\n"}[typ == "full"])
	start := time.Now()
	_, code, err := run(ctx, exe, args...)
	if err != nil {
		return Result{OK: false, Output: log.String() + err.Error()}
	}
	log.WriteString(fmt.Sprintf("Fertig nach %s (Code %d).\n\n", time.Since(start).Round(time.Second), code))
	summary := readNewLog(logPath, before)
	if summary == "" {
		summary = "(Kein Protokoll gefunden: " + logPath + ")"
	}
	log.WriteString(summary)
	return Result{OK: true, Output: log.String()}
}

func download(ctx context.Context, url, dst string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("Server antwortet mit %s", resp.Status)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, io.LimitReader(resp.Body, 1<<30))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// readNewLog liest den neuen Teil von msert.log (meist UTF-16) und kürzt auf das Ergebnis.
func readNewLog(path string, from int64) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if from > 0 && from < int64(len(b)) {
		b = b[from:]
	}
	var text string
	zeros := 0
	for i := 1; i < len(b) && i < 400; i += 2 {
		if b[i] == 0 {
			zeros++
		}
	}
	if (len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE) || zeros > 50 {
		if len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE {
			b = b[2:]
		}
		u := make([]uint16, len(b)/2)
		for i := range u {
			u[i] = binary.LittleEndian.Uint16(b[i*2:])
		}
		text = string(utf16.Decode(u))
	} else {
		text = decodeConsole(b)
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if i := strings.LastIndex(text, "Results Summary"); i >= 0 {
		text = text[i:]
	}
	text = strings.TrimSpace(text)
	if len(text) > 6000 {
		text = "…\n" + text[len(text)-6000:]
	}
	return text
}
