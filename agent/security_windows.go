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
if (-not (Get-MpComputerStatus).RealTimeProtectionEnabled) { [void]$bad.Add('Echtzeitschutz ist aus – bitte am PC unter Windows-Sicherheit einschalten (geht aus Sicherheitsgründen nur dort).') }
if ($env:NRMM_PUA -eq '1') { Set-Nrmm 'Schutz vor unerwünschten Programmen (PUA): an' { Set-MpPreference -PUAProtection Enabled } }
if ($env:NRMM_CLOUD -eq '1') { Set-Nrmm 'Cloud-Schutz: an' { Set-MpPreference -MAPSReporting Advanced -SubmitSamplesConsent SendSafeSamples } }
if ($env:NRMM_NETWORK -eq '1') { Set-Nrmm 'Netzwerkschutz (gefährliche Webseiten): an' { Set-MpPreference -EnableNetworkProtection Enabled } }
if ($env:NRMM_RANSOMWARE -eq '1') { Set-Nrmm 'Ransomware-Schutz (überwachter Ordnerzugriff): an' { Set-MpPreference -EnableControlledFolderAccess Enabled } }
$ok | ForEach-Object { "✓ $_" }
$bad | ForEach-Object { "✗ $_" }
if ($bad.Count -gt 0) { 'Hinweis: Ist der Manipulationsschutz aktiv oder ein anderer Virenscanner installiert, lassen sich manche Einstellungen nur direkt am PC ändern.' }
if ($ok.Count -eq 0 -and $bad.Count -eq 0) { 'Nichts zu tun – ausgewählte Schutzfunktionen sind schon an. Ausschalten geht nur am PC.' }
Write-NrmmData (Get-NrmmSecurity)
if ($ok.Count -eq 0 -and $bad.Count -gt 0) { exit 1 }
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
	return Result{OK: true, Output: out, Data: data}
}

// Ältere Versionen konnten den Echtzeitschutz für Reparaturen pausieren. Diese Funktion gibt es
// nicht mehr: Ein Programm, das den Virenschutz abschalten kann, wird zu Recht misstrauisch beäugt.
// Eine noch laufende alte Pause beendet die damals angelegte Aufgabe selbst.
const taskResume = `NicolajRMM\Schutz-wieder-an`

func pauseFile() string { return filepath.Join(dataDir(), "pause.json") }

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
