//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

// ---------- Pfade ----------

func programDir() string {
	pf := os.Getenv("ProgramFiles")
	if pf == "" {
		pf = `C:\Program Files`
	}
	return filepath.Join(pf, "NicolajRMM")
}

func dataDir() string {
	pd := os.Getenv("ProgramData")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	return filepath.Join(pd, "NicolajRMM")
}

func agentConfigPath() string  { return filepath.Join(dataDir(), "agent.json") }
func screenConfigPath() string { return filepath.Join(dataDir(), "screen.json") }

func logPath(role string) string {
	if role == "screen" {
		// Der Helfer läuft als normaler Benutzer – Logbuch im eigenen Profil.
		if la := os.Getenv("LOCALAPPDATA"); la != "" {
			d := filepath.Join(la, "NicolajRMM")
			os.MkdirAll(d, 0o755)
			return filepath.Join(d, "screen.log")
		}
		return ""
	}
	d := filepath.Join(dataDir(), "logs")
	os.MkdirAll(d, 0o755)
	return filepath.Join(d, "agent.log")
}

func runSim([]string) {
	msgBox("Der Simulationsmodus ist nur für Tests unter Linux/macOS gedacht.", mbIconInfo)
}

// hidden: Kindprozesse ohne aufblitzendes Konsolenfenster starten.
func hidden() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}

func sys32(name string) string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", name)
}

func run(ctx context.Context, exe string, args ...string) (string, int, error) {
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.SysProcAttr = hidden()
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	out := decodeConsole(buf.Bytes())
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return out, ee.ExitCode(), nil
	}
	if err != nil {
		return out, -1, err
	}
	return out, 0, nil
}

// decodeConsole: Windows-Programme schreiben teils UTF-8, teils die alte OEM-Codepage (850).
func decodeConsole(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var sb strings.Builder
	for _, c := range b {
		if c < 0x80 {
			sb.WriteByte(c)
		} else {
			sb.WriteRune(cp850[c-0x80])
		}
	}
	return sb.String()
}

var cp850 = []rune("ÇüéâäàåçêëèïîìÄÅÉæÆôöòûùÿÖÜø£Ø×ƒáíóúñÑªº¿®¬½¼¡«»░▒▓│┤ÁÂÀ©╣║╗╝¢¥┐└┴┬├─┼ãÃ╚╔╩╦╠═╬¤ðÐÊËÈıÍÎÏ┘┌█▄¦Ì▀ÓßÔÒõÕµþÞÚÛÙýÝ¯´\u00ad±‗¾¶§÷¸°¨·¹³²■\u00a0")

// ---------- PowerShell ----------

const psPrelude = `$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Console]::OutputEncoding = [Text.Encoding]::UTF8
$OutputEncoding = [Text.Encoding]::UTF8
function Write-NrmmData($json) { Write-Output '##NRMMDATA##'; Write-Output $json }
function Get-NrmmAdmins {
  try {
    $g = (New-Object Security.Principal.SecurityIdentifier 'S-1-5-32-544').Translate([Security.Principal.NTAccount]).Value.Split('\')[-1]
    return @(([ADSI]"WinNT://$env:COMPUTERNAME/$g,group").psbase.Invoke('Members') | ForEach-Object { $_.GetType().InvokeMember('Name','GetProperty',$null,$_,$null) })
  } catch { return @() }
}
function Get-NrmmUsers {
  $admins = Get-NrmmAdmins
  $list = @(Get-LocalUser | ForEach-Object {
    [pscustomobject]@{
      name = $_.Name; fullName = $_.FullName; description = $_.Description
      enabled = [bool]$_.Enabled; admin = ($admins -contains $_.Name)
      source = [string]$_.PrincipalSource
      lastLogon = $(if ($_.LastLogon) { $_.LastLogon.ToString('yyyy-MM-ddTHH:mm:ss') } else { $null })
      passwordLastSet = $(if ($_.PasswordLastSet) { $_.PasswordLastSet.ToString('yyyy-MM-ddTHH:mm:ss') } else { $null })
      builtin = ($_.SID.Value -match '-(500|501|503|504)$')
    }
  })
  ConvertTo-Json -InputObject $list -Depth 3 -Compress
}
function Set-NrmmMustChange($name) {
  $a = [ADSI]"WinNT://$env:COMPUTERNAME/$name,user"; $a.PasswordExpired = 1; $a.SetInfo()
}
function Get-NrmmApps {
  $keys = @('HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*',
            'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*',
            'Registry::HKEY_USERS\*\Software\Microsoft\Windows\CurrentVersion\Uninstall\*')
  $apps = @(Get-ItemProperty -Path $keys -ErrorAction SilentlyContinue |
    Where-Object { $_.DisplayName -and $_.SystemComponent -ne 1 -and -not $_.ParentKeyName -and -not $_.ReleaseType } |
    ForEach-Object { [pscustomobject]@{ name = [string]$_.DisplayName; version = [string]$_.DisplayVersion; publisher = [string]$_.Publisher; installDate = [string]$_.InstallDate } } |
    Sort-Object name -Unique)
  ConvertTo-Json -InputObject $apps -Depth 3 -Compress
}
`

var psScripts = map[string]string{
	"inventory": `
$os = Get-CimInstance Win32_OperatingSystem
$cs = Get-CimInstance Win32_ComputerSystem
$cpu = Get-CimInstance Win32_Processor | Select-Object -First 1
$bios = Get-CimInstance Win32_BIOS
$disks = @(Get-CimInstance Win32_LogicalDisk -Filter 'DriveType=3' | ForEach-Object {
  [pscustomobject]@{ name = $_.DeviceID; sizeGB = [math]::Round($_.Size/1GB,1); freeGB = [math]::Round($_.FreeSpace/1GB,1) } })
$ips = @()
try { $ips = @(Get-NetIPAddress -AddressFamily IPv4 -ErrorAction Stop | Where-Object { $_.IPAddress -notlike '127.*' -and $_.IPAddress -notlike '169.254.*' } | Select-Object -ExpandProperty IPAddress) } catch {}
$def = $null
try {
  $m = Get-MpComputerStatus -ErrorAction Stop
  $pr = $null; try { $pr = Get-MpPreference -ErrorAction Stop } catch {}
  $act = 0; try { $act = @(Get-MpThreat -ErrorAction Stop | Where-Object { $_.IsActive }).Count } catch {}
  $age = { param($v) if ($null -eq $v) { -1 } elseif ([long]$v -gt 36500) { -1 } else { [long]$v } }
  $def = [pscustomobject]@{
    enabled = [bool]$m.AntivirusEnabled; realtime = [bool]$m.RealTimeProtectionEnabled
    signatureAgeDays = (& $age $m.AntivirusSignatureAge); quickScanAgeDays = (& $age $m.QuickScanAge)
    activeThreats = $act; tamper = [bool]$m.IsTamperProtected; mode = [string]$m.AMRunningMode
    pua = $(if ($pr) { [int]$pr.PUAProtection } else { -1 })
  }
} catch {}
$av = @()
try { $av = @(Get-CimInstance -Namespace root/SecurityCenter2 -ClassName AntiVirusProduct -ErrorAction Stop | ForEach-Object { $h = '{0:x6}' -f [int]$_.productState; [pscustomobject]@{ name = [string]$_.displayName; enabled = ($h.Substring(2,2) -in '10','11') } }) } catch {}
$hf = $null
try { $h = Get-HotFix -ErrorAction Stop | Where-Object InstalledOn | Sort-Object InstalledOn -Descending | Select-Object -First 1; if ($h) { $hf = "$($h.HotFixID) ($($h.InstalledOn.ToString('yyyy-MM-dd')))" } } catch {}
$reboot = (Test-Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired') -or (Test-Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Component Based Servicing\RebootPending')
$inv = [pscustomobject]@{
  hostname = $env:COMPUTERNAME; os = $os.Caption; osVersion = $os.Version; build = $os.BuildNumber
  manufacturer = $cs.Manufacturer; model = $cs.Model; serial = $bios.SerialNumber
  cpu = ([string]$cpu.Name).Trim(); cores = $cpu.NumberOfLogicalProcessors
  ramGB = [math]::Round($cs.TotalPhysicalMemory/1GB,1); ramFreeGB = [math]::Round($os.FreePhysicalMemory*1KB/1GB,1)
  uptimeHours = [math]::Round(((Get-Date) - $os.LastBootUpTime).TotalHours,1)
  loggedOnUser = $cs.UserName; disks = $disks; ips = $ips; domain = $cs.Domain; partOfDomain = [bool]$cs.PartOfDomain
  defender = $def; antivirus = $av; rebootPending = [bool]$reboot; lastHotfix = $hf; agentVersion = $env:NRMM_VERSION
}
Write-NrmmData (ConvertTo-Json -InputObject $inv -Depth 4 -Compress)
`,
	"users.list": `Write-NrmmData (Get-NrmmUsers)`,
	"users.create": `
$u = $env:NRMM_USER
$p = @{ Name = $u; Description = 'Angelegt über Nicolaj Fernwartung'; AccountNeverExpires = $true
        Password = (ConvertTo-SecureString $env:NRMM_PASSWORD -AsPlainText -Force) }
if ($env:NRMM_FULLNAME) { $p.FullName = $env:NRMM_FULLNAME }
if ($env:NRMM_NEVEREXPIRES -eq '1' -and $env:NRMM_MUSTCHANGE -ne '1') { $p.PasswordNeverExpires = $true }
New-LocalUser @p | Out-Null
try { Add-LocalGroupMember -SID 'S-1-5-32-545' -Member $u } catch [Microsoft.PowerShell.Commands.MemberExistsException] {}
if ($env:NRMM_ADMIN -eq '1') { try { Add-LocalGroupMember -SID 'S-1-5-32-544' -Member $u } catch [Microsoft.PowerShell.Commands.MemberExistsException] {} }
if ($env:NRMM_MUSTCHANGE -eq '1') { Set-NrmmMustChange $u }
"Benutzer '$u' wurde angelegt."
Write-NrmmData (Get-NrmmUsers)
`,
	"users.resetPassword": `
$u = $env:NRMM_USER
$lu = Get-LocalUser -Name $u
if ([string]$lu.PrincipalSource -eq 'MicrosoftAccount') { throw "'$u' ist ein Microsoft-Konto. Das Passwort kann nur online unter account.microsoft.com zurückgesetzt werden." }
Set-LocalUser -Name $u -Password (ConvertTo-SecureString $env:NRMM_PASSWORD -AsPlainText -Force)
try { $a = [ADSI]"WinNT://$env:COMPUTERNAME/$u,user"; if ($a.IsAccountLocked) { $a.IsAccountLocked = $false; $a.SetInfo(); 'Kontosperre aufgehoben.' } } catch {}
if ($env:NRMM_MUSTCHANGE -eq '1') { Set-LocalUser -Name $u -PasswordNeverExpires $false; Set-NrmmMustChange $u; 'Muss das Passwort bei der nächsten Anmeldung ändern.' }
"Passwort für '$u' wurde zurückgesetzt."
Write-NrmmData (Get-NrmmUsers)
`,
	"users.setEnabled": `
$u = $env:NRMM_USER
if ($env:NRMM_ENABLED -eq '1') {
  Enable-LocalUser -Name $u; "Benutzer '$u' ist aktiviert."
} else {
  $admins = Get-NrmmAdmins
  if ($admins -contains $u) {
    $others = @(Get-LocalUser | Where-Object { $_.Enabled -and $_.Name -ne $u -and ($admins -contains $_.Name) })
    if ($others.Count -eq 0) { throw "'$u' ist der letzte aktive Administrator und kann nicht deaktiviert werden." }
  }
  Disable-LocalUser -Name $u; "Benutzer '$u' ist deaktiviert."
}
Write-NrmmData (Get-NrmmUsers)
`,
	"users.setAdmin": `
$u = $env:NRMM_USER
if ($env:NRMM_ADMIN -eq '1') {
  try { Add-LocalGroupMember -SID 'S-1-5-32-544' -Member $u } catch [Microsoft.PowerShell.Commands.MemberExistsException] {}
  "'$u' ist jetzt Administrator."
} else {
  $admins = Get-NrmmAdmins
  $others = @(Get-LocalUser | Where-Object { $_.Enabled -and $_.Name -ne $u -and ($admins -contains $_.Name) })
  if ($others.Count -eq 0) { throw "'$u' ist der letzte aktive Administrator. Mindestens ein Administrator muss bleiben." }
  try { Remove-LocalGroupMember -SID 'S-1-5-32-544' -Member $u } catch [Microsoft.PowerShell.Commands.MemberNotFoundException] {}
  "'$u' ist jetzt Standardbenutzer."
}
Write-NrmmData (Get-NrmmUsers)
`,
	"users.delete": `
$u = $env:NRMM_USER
$lu = Get-LocalUser -Name $u
$sid = $lu.SID.Value
if ($sid -match '-(500|501|503|504)$') { throw 'Eingebaute Windows-Konten können nicht gelöscht werden.' }
$admins = Get-NrmmAdmins
if ($admins -contains $u) {
  $others = @(Get-LocalUser | Where-Object { $_.Enabled -and $_.Name -ne $u -and ($admins -contains $_.Name) })
  if ($others.Count -eq 0) { throw "'$u' ist der letzte aktive Administrator und kann nicht gelöscht werden." }
}
Remove-LocalUser -Name $u
"Benutzer '$u' wurde gelöscht."
if ($env:NRMM_DELPROFILE -eq '1') {
  $prof = Get-CimInstance Win32_UserProfile | Where-Object { $_.SID -eq $sid }
  if ($prof) { $prof | Remove-CimInstance; 'Profilordner mit allen Dateien wurde gelöscht.' } else { 'Kein Profilordner vorhanden.' }
}
Write-NrmmData (Get-NrmmUsers)
`,
	"software.list": `Write-NrmmData (Get-NrmmApps)`,
}

// runPS führt ein PowerShell-Skript aus. Werte (z. B. Passwörter) kommen nur über
// Umgebungsvariablen hinein – so kann keine Eingabe als Befehl interpretiert werden.
func runPS(ctx context.Context, script string, env map[string]string, prelude bool) (string, json.RawMessage, error) {
	tmpDir := filepath.Join(dataDir(), "tmp")
	os.MkdirAll(tmpDir, 0o700)
	f, err := os.CreateTemp(tmpDir, "nrmm-*.ps1")
	if err != nil {
		return "", nil, err
	}
	path := f.Name()
	defer os.Remove(path)
	body := script
	if prelude {
		body = psPrelude + script
	} else {
		body = "[Console]::OutputEncoding = [Text.Encoding]::UTF8\r\n$ProgressPreference = 'SilentlyContinue'\r\n" + script
	}
	f.Write([]byte{0xEF, 0xBB, 0xBF}) // UTF-8 mit BOM, damit Umlaute im Skript stimmen
	f.WriteString(strings.ReplaceAll(body, "\r\n", "\n"))
	f.Close()

	ps := sys32(`WindowsPowerShell\v1.0\powershell.exe`)
	cmd := exec.CommandContext(ctx, ps, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", path)
	cmd.SysProcAttr = hidden()
	cmd.Env = append(os.Environ(), "NRMM_VERSION="+agentVersion)
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	runErr := cmd.Run()
	out := decodeConsole(buf.Bytes())
	var data json.RawMessage
	if i := strings.Index(out, "##NRMMDATA##"); i >= 0 {
		js := strings.TrimSpace(out[i+len("##NRMMDATA##"):])
		if json.Valid([]byte(js)) {
			data = json.RawMessage(js)
		}
		out = out[:i]
	}
	out = strings.TrimSpace(out)
	if ctx.Err() == context.DeadlineExceeded {
		return out, data, errors.New("Zeitüberschreitung")
	}
	if runErr != nil {
		if out == "" {
			out = runErr.Error()
		}
		return out, data, runErr
	}
	return out, data, nil
}

// ---------- winget ----------

func findWinget() (string, error) {
	pattern := filepath.Join(os.Getenv("ProgramFiles"), "WindowsApps", "Microsoft.DesktopAppInstaller_*_8wekyb3d8bbwe", "winget.exe")
	matches, _ := filepath.Glob(pattern)
	if len(matches) > 0 {
		sort.Slice(matches, func(i, j int) bool { return versionLess(pkgVersion(matches[j]), pkgVersion(matches[i])) })
		return matches[0], nil
	}
	if p, err := exec.LookPath("winget.exe"); err == nil {
		return p, nil
	}
	return "", errors.New("winget ist auf diesem PC nicht verfügbar. Bitte im Microsoft Store den „App-Installer“ installieren bzw. aktualisieren.")
}

func pkgVersion(path string) []int {
	dir := filepath.Base(filepath.Dir(path))
	parts := strings.Split(dir, "_")
	if len(parts) < 2 {
		return nil
	}
	var v []int
	for _, s := range strings.Split(parts[1], ".") {
		n, _ := strconv.Atoi(s)
		v = append(v, n)
	}
	return v
}

func versionLess(a, b []int) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

func runWinget(ctx context.Context, args ...string) (string, uint32, error) {
	wg, err := findWinget()
	if err != nil {
		return "", 0, err
	}
	cmd := exec.CommandContext(ctx, wg, args...)
	cmd.Dir = filepath.Dir(wg)
	cmd.SysProcAttr = hidden()
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	runErr := cmd.Run()
	out := cleanOutput(decodeConsole(buf.Bytes()))
	var ee *exec.ExitError
	if errors.As(runErr, &ee) {
		return out, uint32(ee.ExitCode()), nil
	}
	if runErr != nil {
		return out, 0, runErr
	}
	return out, 0, nil
}

const (
	wgNoApplicableInstaller = 0x8A150010
	wgUpdateNotApplicable   = 0x8A15002B
	wgAlreadyInstalled      = 0x8A150061
	wgNoPackageFound        = 0x8A150014
)

var wingetCommon = []string{"--source", "winget", "--accept-source-agreements", "--disable-interactivity"}

func wingetResult(out string, code uint32, err error, okMsg string) Result {
	if err != nil {
		return Result{OK: false, Output: err.Error()}
	}
	switch code {
	case 0:
		return Result{OK: true, Output: strings.TrimSpace(okMsg + "\n\n" + out)}
	case wgUpdateNotApplicable, wgAlreadyInstalled:
		return Result{OK: true, Output: "Bereits installiert und aktuell.\n\n" + out}
	case wgNoPackageFound:
		return Result{OK: false, Output: "Paket nicht gefunden.\n\n" + out}
	}
	return Result{OK: false, Output: fmt.Sprintf("winget meldet Fehler 0x%08X\n\n%s", code, out)}
}

// ---------- Aufträge ausführen ----------

func execCommand(ctx context.Context, cmd Cmd) Result {
	a := map[string]any{}
	json.Unmarshal(cmd.Args, &a)
	user := argString(a, "username")
	if strings.HasPrefix(cmd.Type, "users.") && cmd.Type != "users.list" && !reUser.MatchString(user) {
		return Result{OK: false, Output: "Ungültiger Benutzername."}
	}
	ps := func(env map[string]string) Result {
		out, data, err := runPS(ctx, psScripts[cmd.Type], env, true)
		if err != nil {
			return Result{OK: false, Output: cleanPSError(out), Data: data}
		}
		return Result{OK: true, Output: out, Data: data}
	}
	refreshApps := func(r Result) Result {
		if _, data, err := runPS(context.Background(), psScripts["software.list"], nil, true); err == nil {
			r.Data = data
		}
		return r
	}

	if strings.HasPrefix(cmd.Type, "security.") {
		return execSecurity(ctx, cmd, a)
	}

	switch cmd.Type {
	case "inventory", "users.list", "software.list":
		return ps(nil)
	case "users.create":
		return ps(map[string]string{
			"NRMM_USER": user, "NRMM_PASSWORD": argString(a, "password"), "NRMM_FULLNAME": argString(a, "fullName"),
			"NRMM_ADMIN": boolEnv(argBool(a, "admin")), "NRMM_MUSTCHANGE": boolEnv(argBool(a, "mustChange")),
			"NRMM_NEVEREXPIRES": boolEnv(argBool(a, "neverExpires")),
		})
	case "users.resetPassword":
		return ps(map[string]string{"NRMM_USER": user, "NRMM_PASSWORD": argString(a, "password"), "NRMM_MUSTCHANGE": boolEnv(argBool(a, "mustChange"))})
	case "users.setEnabled":
		return ps(map[string]string{"NRMM_USER": user, "NRMM_ENABLED": boolEnv(argBool(a, "enabled"))})
	case "users.setAdmin":
		return ps(map[string]string{"NRMM_USER": user, "NRMM_ADMIN": boolEnv(argBool(a, "admin"))})
	case "users.delete":
		return ps(map[string]string{"NRMM_USER": user, "NRMM_DELPROFILE": boolEnv(argBool(a, "deleteProfile"))})

	case "software.updates":
		out, code, err := runWinget(ctx, append([]string{"upgrade"}, wingetCommon...)...)
		if err != nil {
			return Result{OK: false, Output: err.Error()}
		}
		rows := parseWingetTable(out, "updates")
		b, _ := json.Marshal(rows)
		if code != 0 && len(rows) == 0 && !strings.Contains(out, "----") {
			return Result{OK: true, Output: out, Data: json.RawMessage("[]")}
		}
		return Result{OK: true, Output: fmt.Sprintf("%d Updates verfügbar.\n\n%s", len(rows), out), Data: b}
	case "software.search":
		q := argString(a, "query")
		out, _, err := runWinget(ctx, append([]string{"search", "--query", q}, wingetCommon...)...)
		if err != nil {
			return Result{OK: false, Output: err.Error()}
		}
		rows := parseWingetTable(out, "search")
		b, _ := json.Marshal(map[string]any{"query": q, "rows": rows})
		return Result{OK: true, Output: fmt.Sprintf("%d Treffer.", len(rows)), Data: b}
	case "software.install":
		id := argString(a, "id")
		if !rePkg.MatchString(id) {
			return Result{OK: false, Output: "Ungültige Paket-ID."}
		}
		base := append([]string{"install", "--id", id, "--exact", "--silent", "--accept-package-agreements"}, wingetCommon...)
		out, code, err := runWinget(ctx, append(base, "--scope", "machine")...)
		if err == nil && code == wgNoApplicableInstaller { // kein Installer für alle Benutzer → Standard probieren
			out, code, err = runWinget(ctx, base...)
		}
		return refreshApps(wingetResult(out, code, err, "Installiert."))
	case "software.upgrade":
		id := argString(a, "id")
		if !rePkg.MatchString(id) {
			return Result{OK: false, Output: "Ungültige Paket-ID."}
		}
		out, code, err := runWinget(ctx, append([]string{"upgrade", "--id", id, "--exact", "--silent", "--accept-package-agreements"}, wingetCommon...)...)
		return refreshApps(wingetResult(out, code, err, "Aktualisiert."))
	case "software.upgradeAll":
		out, code, err := runWinget(ctx, append([]string{"upgrade", "--all", "--silent", "--accept-package-agreements"}, wingetCommon...)...)
		return refreshApps(wingetResult(out, code, err, "Aktualisierung abgeschlossen."))
	case "software.uninstall":
		name := argString(a, "name")
		out, code, err := runWinget(ctx, "uninstall", "--name", name, "--exact", "--silent", "--accept-source-agreements", "--disable-interactivity")
		return refreshApps(wingetResult(out, code, err, "Deinstalliert."))

	case "power.restart", "power.shutdown":
		delay := argInt(a, "delay")
		msg := strings.TrimSpace(argString(a, "message"))
		flag := "/r"
		if msg == "" {
			msg = "Nicolaj Support startet diesen PC neu. Bitte speichere deine Arbeit."
		}
		if cmd.Type == "power.shutdown" {
			flag = "/s"
			if argString(a, "message") == "" {
				msg = "Nicolaj Support fährt diesen PC herunter. Bitte speichere deine Arbeit."
			}
		}
		out, code, err := run(ctx, sys32("shutdown.exe"), flag, "/t", strconv.Itoa(delay), "/c", msg, "/d", "p:0:0")
		if err != nil || code != 0 {
			return Result{OK: false, Output: strings.TrimSpace(fmt.Sprintf("%s %v (Code %d)", out, err, code))}
		}
		return Result{OK: true, Output: fmt.Sprintf("Geplant in %d Sekunden.", delay)}
	case "power.abort":
		out, code, err := run(ctx, sys32("shutdown.exe"), "/a")
		if err != nil || code != 0 {
			return Result{OK: false, Output: strings.TrimSpace(out + " (Es war vermutlich nichts geplant.)")}
		}
		return Result{OK: true, Output: "Abgebrochen."}

	case "script.run":
		out, _, err := runPS(ctx, argString(a, "script"), nil, false)
		if err != nil {
			return Result{OK: false, Output: out}
		}
		if out == "" {
			out = "(keine Ausgabe)"
		}
		return Result{OK: true, Output: out}

	case "agent.uninstall":
		exe, _ := os.Executable()
		go func() {
			time.Sleep(2 * time.Second)
			c := exec.Command(exe, "uninstall", "--quiet")
			c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000 | 0x00000200}
			c.Start()
		}()
		return Result{OK: true, Output: "Der Agent wird jetzt entfernt. Das Gerät verschwindet danach aus dem Dashboard."}
	}
	return Result{OK: false, Output: "Unbekannte Aktion: " + cmd.Type}
}

// cleanPSError kürzt PowerShell-Fehlermeldungen auf das Wesentliche.
func cleanPSError(s string) string {
	var keep []string
	for _, l := range strings.Split(s, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "+ ") || strings.HasPrefix(t, "+") && strings.Contains(t, "~") ||
			strings.HasPrefix(t, "CategoryInfo") || strings.HasPrefix(t, "FullyQualifiedErrorId") ||
			strings.HasPrefix(t, "In ") && strings.Contains(t, ".ps1:") || strings.HasPrefix(t, "At ") && strings.Contains(t, ".ps1:") {
			continue
		}
		keep = append(keep, l)
	}
	return strings.TrimSpace(strings.Join(keep, "\n"))
}

func collectInventory() json.RawMessage {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	_, data, err := runPS(ctx, psScripts["inventory"], nil, true)
	if err != nil {
		logf("Systeminfo: %v", err)
	}
	return data
}
