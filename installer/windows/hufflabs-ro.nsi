; HuffLabs RO installer for Windows 10/11.
; Build: makensis -DSITE_URL=https://<your site> -DVERSION=<YYYY.MM.DD> installer/windows/hufflabs-ro.nsi
; Carries only the Microsoft runtimes; downloads the ~4 GB game from the HuffLabs
; site during install (needs Tailscale connected), using Windows' own curl.exe/tar.exe.

Unicode true
RequestExecutionLevel admin
SetCompressor /SOLID lzma
!include "MUI2.nsh"
!include "x64.nsh"
!include "LogicLib.nsh"
!include "paths.nsh"
!include "registry.nsh"

!ifndef SITE_URL
  !error "Pass the site address: makensis -DSITE_URL=https://… (see docs/setup.md)"
!endif
!ifndef VERSION
  !define VERSION "dev"
!endif
!ifndef LAUNCHER
  !define LAUNCHER "..\..\build\launcher\HuffLabsRO-Launcher.exe"   ; launcher/build.sh
!endif
!define APP "HuffLabs RO"
!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\HuffLabsRO"

Name "${APP}"
OutFile "..\..\dist\HuffLabsRO-Setup.exe"
InstallDir "C:\Games\HuffLabsRO"   ; always ends in \HuffLabsRO, see paths.nsh
InstallDirRegKey HKLM "${UNINST_KEY}" "InstallLocation"
BrandingText "HuffLabs RO ${VERSION}"
ShowInstDetails show

!define MUI_ICON "..\..\theme\out\HuffLabsRO.ico"
!define MUI_UNICON "..\..\theme\out\HuffLabsRO.ico"
!define MUI_WELCOMEPAGE_TITLE "Welcome to HuffLabs RO"
!define MUI_WELCOMEPAGE_TEXT "This installs HuffLabs RO, a private Ragnarok Online server for friends.$\r$\n$\r$\nTailscale must be installed and connected first, because the game (about 4 GB) is downloaded from the HuffLabs server during setup.$\r$\n$\r$\nClick Next to continue."
; Start the game through Explorer so it runs as the user, not elevated like setup.
!define MUI_FINISHPAGE_RUN
!define MUI_FINISHPAGE_RUN_FUNCTION PlayNow
!define MUI_FINISHPAGE_RUN_TEXT "Play HuffLabs RO now"
!insertmacro MUI_PAGE_WELCOME
!define MUI_DIRECTORYPAGE_TEXT_TOP "HuffLabs RO goes in a folder called HuffLabsRO inside the folder you pick."
!define MUI_PAGE_CUSTOMFUNCTION_LEAVE DirLeave
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Var Tools   ; folder holding Windows' 64-bit curl.exe/tar.exe (Sysnative from a 32-bit installer)
Var Ver
Var Want
Var Zip

!insertmacro HLRO_PATH_FUNCS ""
!insertmacro HLRO_PATH_FUNCS "un."
!insertmacro HLRO_REG_FUNCS ""
!insertmacro HLRO_REG_FUNCS "un."

Function PlayNow
  Exec '"$WINDIR\explorer.exe" "$INSTDIR\HuffLabsRO-Launcher.exe"'
FunctionEnd

Function DirLeave
  Call HlroFixInstDir
  Call HlroTargetOK
  ${If} $R0 == 0
    MessageBox MB_ICONEXCLAMATION "$INSTDIR already has other files in it. Pick a different folder." /SD IDOK
    Abort
  ${EndIf}
FunctionEnd

Function .onInit
  StrCpy $Tools "$WINDIR\Sysnative"
  ${IfNot} ${FileExists} "$Tools\curl.exe"
    StrCpy $Tools "$SYSDIR"
  ${EndIf}
FunctionEnd

; Reads the SHA-256 for $Ver's zip from SHA256SUMS at $0 into $Want.
Function FindSum
  StrCpy $Want ""
  FileOpen $1 "$0" r
  loop:
    FileRead $1 $2
    StrCmp $2 "" done
    ; line: "<64 hex>  HuffLabsRO-client-<ver>.zip"
    StrCpy $3 $2 64
    StrCpy $4 $2 "" 66
    StrCpy $5 "HuffLabsRO-client-$Ver.zip"
    StrLen $6 $5
    StrCpy $4 $4 $6
    StrCmp $4 $5 0 loop
    StrCpy $Want $3
  done:
  FileClose $1
FunctionEnd

Section "HuffLabs RO" SecMain
  ; again here for silent installs (/D=), which skip the folder page
  Call HlroFixInstDir
  Call HlroTargetOK
  ${If} $R0 == 0
    MessageBox MB_ICONSTOP "$INSTDIR already has other files in it. Run the installer again and pick a different folder." /SD IDOK
    Abort "$INSTDIR already has other files in it."
  ${EndIf}
  SetOutPath "$INSTDIR"
  InitPluginsDir

  DetailPrint "Checking the HuffLabs RO server..."
  nsExec::ExecToStack '"$Tools\curl.exe" -sf --max-time 15 -o "$PLUGINSDIR\VERSION" "${SITE_URL}/download/VERSION"'
  Pop $0
  ${If} $0 != 0
    MessageBox MB_YESNO|MB_ICONEXCLAMATION "Can't reach the HuffLabs RO server.$\r$\n$\r$\nMake sure Tailscale is installed and connected, then run this again.$\r$\n$\r$\nOpen the Tailscale download page now?" /SD IDNO IDNO +2
      ExecShell "open" "https://tailscale.com/download"
    Abort "Can't reach the HuffLabs RO server."
  ${EndIf}
  FileOpen $1 "$PLUGINSDIR\VERSION" r
  FileRead $1 $Ver
  FileClose $1
  ; strip the trailing newline
  StrCpy $2 $Ver 1 -1
  ${If} $2 == "$\n"
    StrCpy $Ver $Ver -1
  ${EndIf}
  StrCpy $2 $Ver 1 -1
  ${If} $2 == "$\r"
    StrCpy $Ver $Ver -1
  ${EndIf}
  nsExec::ExecToStack '"$Tools\curl.exe" -sf --max-time 15 -o "$PLUGINSDIR\SHA256SUMS" "${SITE_URL}/download/SHA256SUMS"'
  Pop $0
  StrCpy $0 "$PLUGINSDIR\SHA256SUMS"
  Call FindSum
  ${If} $Want == ""
    MessageBox MB_ICONSTOP "The server has no checksum for version $Ver. Try again later." /SD IDOK
    Abort
  ${EndIf}

  DetailPrint "Installing Microsoft Visual C++ and DirectX runtimes..."
  File "/oname=$PLUGINSDIR\vc_redist.x86.exe" "redist\vc_redist.x86.exe"
  ExecWait '"$PLUGINSDIR\vc_redist.x86.exe" /install /quiet /norestart'
  ${If} ${RunningX64}
    File "/oname=$PLUGINSDIR\vc_redist.x64.exe" "redist\vc_redist.x64.exe"
    ExecWait '"$PLUGINSDIR\vc_redist.x64.exe" /install /quiet /norestart'
  ${EndIf}
  File "/oname=$PLUGINSDIR\directx_Jun2010_redist.exe" "redist\directx_Jun2010_redist.exe"
  ExecWait '"$PLUGINSDIR\directx_Jun2010_redist.exe" /Q /T:"$PLUGINSDIR\dx"'
  ExecWait '"$PLUGINSDIR\dx\DXSETUP.exe" /silent'

  Call HlroLauncherInstalled
  ${If} $R0 == 1
  DetailPrint "HuffLabs RO is already installed here; the launcher keeps it up to date."
  ${Else}
  DetailPrint "Downloading HuffLabs RO $Ver (about 4 GB, this takes a while)..."
  CreateDirectory "$INSTDIR\.download"
  StrCpy $Zip "$INSTDIR\.download\HuffLabsRO-client-$Ver.zip"
  ; -C - resumes a download interrupted on an earlier run; under 1 KB/s for a
  ; minute counts as interrupted. It runs in its own console window so the friend
  ; sees curl's progress bar (nsExec would hide it).
  DetailPrint "A window shows the download's progress. Leave it open."
  StrCpy $R9 "error"
  ClearErrors
  ExecWait '"$Tools\curl.exe" -fL -C - --retry 5 --retry-delay 3 --speed-limit 1024 --speed-time 60 --progress-bar -o "$Zip" "${SITE_URL}/download/HuffLabsRO-client-$Ver.zip"' $R9
  ${If} ${Errors}
    StrCpy $R9 "error"
  ${EndIf}

  DetailPrint "Checking the download..."
  ; the path goes through an environment variable, so quotes or apostrophes in the
  ; folder name can't break the PowerShell command
  System::Call 'Kernel32::SetEnvironmentVariable(t "HLRO_ZIP", t "$Zip")i'
  nsExec::ExecToStack '"$Tools\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -NonInteractive -Command "(Get-FileHash -Algorithm SHA256 -LiteralPath $$env:HLRO_ZIP).Hash"'
  Pop $0
  Pop $1
  StrCpy $1 $1 64
  ${If} $1 != $Want   ; StrCmp/!= is case-insensitive in NSIS
    ${If} $R9 != 0
      ; keep the partial file so the next run carries on from it (curl exits 0
      ; when asked to resume a file that is already complete, so a complete but
      ; corrupt file still ends up deleted below)
      MessageBox MB_ICONEXCLAMATION "The download was interrupted (curl exit code $R9). Run the installer again to continue where it stopped." /SD IDOK
      Abort "The download was interrupted."
    ${EndIf}
    Delete "$Zip"
    MessageBox MB_ICONSTOP "The download was damaged. Run the installer again to retry." /SD IDOK
    Abort "The download was damaged."
  ${EndIf}

  DetailPrint "Unpacking..."
  Push "$INSTDIR\.unpack"
  Call HlroRmTree
  CreateDirectory "$INSTDIR\.unpack"
  nsExec::ExecToLog '"$Tools\tar.exe" -xf "$Zip" -C "$INSTDIR\.unpack"'
  Pop $0
  ${If} $0 != 0
    MessageBox MB_ICONSTOP "Unpacking failed (tar exit code $0). Run the installer again." /SD IDOK
    Abort
  ${EndIf}
  ; Move the HuffLabsRO\ folder's contents into the install folder; savedata\ isn't in
  ; the archive, so a player's settings survive an update.
  nsExec::ExecToLog '"$Tools\robocopy.exe" "$INSTDIR\.unpack\HuffLabsRO" "$INSTDIR" /E /MOVE /NFL /NDL /NJH /NJS /NP'
  Pop $0
  ; robocopy: 0-7 is success, 8+ is failure; nsExec gives "error" if it couldn't start
  ${If} $0 == "error"
  ${OrIf} $0 >= 8
    MessageBox MB_ICONSTOP "Copying the game files failed (robocopy code $0)." /SD IDOK
    Abort
  ${EndIf}
  Push "$INSTDIR\.unpack"
  Call HlroRmTree
  Push "$INSTDIR\.download"
  Call HlroRmTree
  ${EndIf}

  ; The launcher checks for updates, then starts the game. Friends run it as
  ; themselves, so give normal users write access to the game folder.
  File "/oname=$INSTDIR\HuffLabsRO-Launcher.exe" "${LAUNCHER}"
  nsExec::ExecToLog '"$Tools\icacls.exe" "$INSTDIR" /grant *S-1-5-32-545:(OI)(CI)M /T /C /Q'
  Pop $0
  ${If} $0 != 0
    DetailPrint "Couldn't give users write access to $INSTDIR (icacls code $0); updates may need the game run as administrator."
  ${EndIf}

  DetailPrint "Writing default game settings..."
  Call HlroWriteGameKeys
  CreateDirectory "$INSTDIR\savedata"
  ${IfNot} ${FileExists} "$INSTDIR\savedata\OptionInfo.lua"
    CopyFiles /SILENT "$INSTDIR\defaults\OptionInfo.lua" "$INSTDIR\savedata\OptionInfo.lua"
  ${EndIf}

  DetailPrint "Adding shortcuts..."
  CreateShortcut "$DESKTOP\${APP}.lnk" "$INSTDIR\HuffLabsRO-Launcher.exe" "" "$INSTDIR\HuffLabsRO.ico" 0
  CreateDirectory "$SMPROGRAMS\${APP}"
  CreateShortcut "$SMPROGRAMS\${APP}\${APP}.lnk" "$INSTDIR\HuffLabsRO-Launcher.exe" "" "$INSTDIR\HuffLabsRO.ico" 0
  CreateShortcut "$SMPROGRAMS\${APP}\Game settings (OpenSetup).lnk" "$INSTDIR\opensetup.exe" "" "$INSTDIR\HuffLabsRO.ico" 0
  CreateShortcut "$SMPROGRAMS\${APP}\Uninstall ${APP}.lnk" "$INSTDIR\Uninstall.exe"

  WriteUninstaller "$INSTDIR\Uninstall.exe"
  ; The uninstaller runs elevated from a folder users can write to, so users get
  ; read-only access to it (admins and SYSTEM keep full control).
  nsExec::ExecToLog '"$Tools\icacls.exe" "$INSTDIR\Uninstall.exe" /inheritance:r /grant:r *S-1-5-32-544:F *S-1-5-18:F *S-1-5-32-545:RX /Q'
  Pop $0
  WriteRegStr   HKLM "${UNINST_KEY}" "DisplayName" "${APP}"
  WriteRegStr   HKLM "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\HuffLabsRO.ico"
  WriteRegStr   HKLM "${UNINST_KEY}" "DisplayVersion" "$Ver"
  WriteRegStr   HKLM "${UNINST_KEY}" "Publisher" "HuffLabs"
  WriteRegStr   HKLM "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr   HKLM "${UNINST_KEY}" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegDWORD HKLM "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKLM "${UNINST_KEY}" "NoRepair" 1
SectionEnd

Section "Uninstall"
  ; The Visual C++ and DirectX runtimes are shared with other programs and stay.
  Delete "$DESKTOP\${APP}.lnk"
  Push "$SMPROGRAMS\${APP}"
  Call un.HlroRmTree
  Call un.HlroRemoveInstall
  ${If} $R0 == 1
    MessageBox MB_ICONINFORMATION "The game files were left in $INSTDIR, because that folder may hold other things. Delete the HuffLabs RO files there yourself." /SD IDOK
  ${EndIf}
  Call un.HlroRemoveGameKeys   ; reads the uninstall key, so before deleting it
  DeleteRegKey HKLM "${UNINST_KEY}"
SectionEnd
