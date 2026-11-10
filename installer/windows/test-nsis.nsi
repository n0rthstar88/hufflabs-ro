; Tests paths.nsh and registry.nsh under Wine. Run: bash installer/windows/test-nsis.sh
Unicode true
SilentInstall silent
RequestExecutionLevel user
OutFile "${OUT}"
!include "LogicLib.nsh"
!include "paths.nsh"
!insertmacro HLRO_PATH_FUNCS ""
; registry tests use their own keys, not the real Gravity ones
!define HLRO_GRAVITY_SOFT "Software\HLROTest\Gravity Soft"
!define HLRO_GRAVITY "Software\HLROTest\Gravity"
!define HLRO_STATE_KEY "Software\HLROTest\State"
!include "registry.nsh"
!insertmacro HLRO_REG_FUNCS ""

Var Out
Var Fails

!macro EQ name got want
  ${If} "${got}" == "${want}"
    FileWrite $Out "ok   ${name}$\r$\n"
  ${Else}
    FileWrite $Out "FAIL ${name}: got [${got}] want [${want}]$\r$\n"
    IntOp $Fails $Fails + 1
  ${EndIf}
!macroend

!macro FIX in want
  StrCpy $INSTDIR "${in}"
  Call HlroFixInstDir
  !insertmacro EQ "fix ${in}" $INSTDIR "${want}"
!macroend

Section
  StrCpy $Fails 0
  FileOpen $Out "$EXEDIR\results.txt" w
  StrCpy $9 "$EXEDIR\cases"
  RMDir /r $9

  ; the install folder always ends in \HuffLabsRO
  !insertmacro FIX "C:\Games" "C:\Games\HuffLabsRO"
  !insertmacro FIX "C:\Games\" "C:\Games\HuffLabsRO"
  !insertmacro FIX "C:\" "C:\HuffLabsRO"
  !insertmacro FIX "C:\Games\HuffLabsRO" "C:\Games\HuffLabsRO"
  !insertmacro FIX "C:\Games\hufflabsro\" "C:\Games\hufflabsro"
  !insertmacro FIX "D:\Spiele für mich" "D:\Spiele für mich\HuffLabsRO"

  ; installing refuses a non-empty folder that isn't a HuffLabs RO install
  CreateDirectory "$9\new\HuffLabsRO"
  StrCpy $INSTDIR "$9\new\HuffLabsRO"
  Call HlroTargetOK
  !insertmacro EQ "empty folder ok" $R0 "1"
  StrCpy $INSTDIR "$9\missing\HuffLabsRO"
  Call HlroTargetOK
  !insertmacro EQ "missing folder ok" $R0 "1"
  CreateDirectory "$9\other\HuffLabsRO"
  FileOpen $1 "$9\other\HuffLabsRO\notes.txt" w
  FileClose $1
  StrCpy $INSTDIR "$9\other\HuffLabsRO"
  Call HlroTargetOK
  !insertmacro EQ "foreign folder refused" $R0 "0"
  CreateDirectory "$9\ours\HuffLabsRO\.download"
  StrCpy $INSTDIR "$9\ours\HuffLabsRO"
  Call HlroTargetOK
  !insertmacro EQ "half-done install ok" $R0 "1"

  ; a folder the launcher already manages counts as ours, and is reported as such
  CreateDirectory "$9\launched\HuffLabsRO\.hufflabs-launcher"
  CreateDirectory "$9\launched\HuffLabsRO\savedata"
  FileOpen $1 "$9\launched\HuffLabsRO\.hufflabs-launcher\state.json" w
  FileClose $1
  StrCpy $INSTDIR "$9\launched\HuffLabsRO"
  Call HlroTargetOK
  !insertmacro EQ "launcher-managed folder ok" $R0 "1"
  Call HlroLauncherInstalled
  !insertmacro EQ "launcher state detected" $R0 "1"
  StrCpy $INSTDIR "$9\new\HuffLabsRO"
  Call HlroLauncherInstalled
  !insertmacro EQ "no launcher state" $R0 "0"

  ; uninstalling only wipes a folder named HuffLabsRO that holds the game
  CreateDirectory "$9\u1\HuffLabsRO\data"
  FileOpen $1 "$9\u1\HuffLabsRO\HuffLabsRO.exe" w
  FileClose $1
  StrCpy $INSTDIR "$9\u1\HuffLabsRO"
  Call HlroRemoveInstall
  ${If} ${FileExists} "$9\u1\HuffLabsRO\*.*"
    !insertmacro EQ "own folder removed" "exists" "gone"
  ${Else}
    !insertmacro EQ "own folder removed" "gone" "gone"
  ${EndIf}
  !insertmacro EQ "own folder: nothing left behind" $R0 "0"

  ; a junction planted inside the install must not lead the (elevated) uninstaller
  ; to delete what it points at
  CreateDirectory "$9\outside"
  FileOpen $1 "$9\outside\keep.txt" w
  FileClose $1
  CreateDirectory "$9\j\HuffLabsRO"
  FileOpen $1 "$9\j\HuffLabsRO\HuffLabsRO.exe" w
  FileClose $1
  nsExec::ExecToStack 'cmd /c mklink /J "$9\j\HuffLabsRO\link" "$9\outside"'
  Pop $2
  Pop $3
  ${If} ${FileExists} "$9\j\HuffLabsRO\link\keep.txt"
    StrCpy $INSTDIR "$9\j\HuffLabsRO"
    Call HlroRemoveInstall
    ${If} ${FileExists} "$9\outside\keep.txt"
      !insertmacro EQ "junction target survives uninstall" "kept" "kept"
    ${Else}
      !insertmacro EQ "junction target survives uninstall" "deleted" "kept"
    ${EndIf}
  ${Else}
    FileWrite $Out "skip junction test (mklink /J unavailable: $2 $3)$\r$\n"
  ${EndIf}

  ; an old-style install straight into C:\Games: other files survive
  CreateDirectory "$9\Games\data"
  FileOpen $1 "$9\Games\HuffLabsRO.exe" w
  FileClose $1
  FileOpen $1 "$9\Games\data.grf" w
  FileClose $1
  FileOpen $1 "$9\Games\Uninstall.exe" w
  FileClose $1
  FileOpen $1 "$9\Games\OtherGame.exe" w
  FileClose $1
  StrCpy $INSTDIR "$9\Games"
  Call HlroRemoveInstall
  ${If} ${FileExists} "$9\Games\OtherGame.exe"
    !insertmacro EQ "other files kept" "kept" "kept"
  ${Else}
    !insertmacro EQ "other files kept" "deleted" "kept"
  ${EndIf}
  ${If} ${FileExists} "$9\Games\Uninstall.exe"
    !insertmacro EQ "uninstaller removed" "kept" "gone"
  ${Else}
    !insertmacro EQ "uninstaller removed" "gone" "gone"
  ${EndIf}
  !insertmacro EQ "foreign folder: files left behind" $R0 "1"

  ; --- registry: a fresh PC gets every key, and uninstall removes them all ---
  SetRegView 32
  DeleteRegKey HKLM "Software\HLROTest"
  SetRegView default
  DeleteRegKey HKLM "Software\HLROTest"
  DeleteRegKey HKCU "Software\HLROTest"
  StrCpy $INSTDIR "C:\Games\HuffLabsRO"
  Call HlroWriteGameKeys
  SetRegView 32
  ReadRegDWORD $2 HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok" "SOUNDMODE"
  ClearErrors
  ReadRegStr $3 HKLM "${HLRO_GRAVITY_SOFT}\RenewSetup" ""
  ${If} ${Errors}
    StrCpy $3 "missing"
  ${EndIf}
  SetRegView default
  !insertmacro EQ "fresh: settings written" $2 "1"
  !insertmacro EQ "fresh: RenewSetup written" $3 ""
  ReadRegStr $2 HKCU "${HLRO_GRAVITY}\RagnarokOnline" "RagPath"
  !insertmacro EQ "fresh: RagPath points here" $2 "C:\Games\HuffLabsRO\"
  ; reinstall must not lose the record that we created the keys
  Call HlroWriteGameKeys
  Call HlroRemoveGameKeys
  SetRegView 32
  ClearErrors
  EnumRegValue $2 HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok" 0
  StrCpy $3 $2
  ClearErrors
  ReadRegStr $2 HKLM "${HLRO_GRAVITY_SOFT}\RenewSetup" ""
  ${If} ${Errors}
    StrCpy $2 "gone"
  ${EndIf}
  SetRegView default
  !insertmacro EQ "fresh uninstall: settings key gone" $3 ""
  !insertmacro EQ "fresh uninstall: RenewSetup gone" $2 "gone"
  ClearErrors
  ReadRegStr $2 HKCU "${HLRO_GRAVITY}\RagnarokOnline" "RagPath"
  ${If} ${Errors}
    StrCpy $2 "gone"
  ${EndIf}
  !insertmacro EQ "fresh uninstall: RagPath gone" $2 "gone"

  ; --- registry: another RO client already set things up ---
  DeleteRegKey HKLM "${HLRO_STATE_KEY}"
  SetRegView 32
  WriteRegDWORD HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok" "SOUNDMODE" 0
  WriteRegStr HKLM "${HLRO_GRAVITY_SOFT}\RenewSetup" "" ""
  SetRegView default
  WriteRegStr HKCU "${HLRO_GRAVITY}\RagnarokOnline" "RagPath" "D:\OtherRO\"
  WriteRegStr HKCU "${HLRO_GRAVITY}\RagnarokOnline" "SakrayPath" "D:\OtherRO\"
  WriteRegStr HKCU "${HLRO_GRAVITY}\RagnarokOnline" "" "D:\OtherRO\"
  Call HlroWriteGameKeys
  SetRegView 32
  ReadRegDWORD $2 HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok" "SOUNDMODE"
  SetRegView default
  !insertmacro EQ "existing: other client's settings kept" $2 "0"
  ReadRegStr $2 HKCU "${HLRO_GRAVITY}\RagnarokOnline" "RagPath"
  !insertmacro EQ "existing: RagPath points here while installed" $2 "C:\Games\HuffLabsRO\"
  Call HlroWriteGameKeys   ; a reinstall must not back up our own path
  Call HlroRemoveGameKeys
  SetRegView 32
  ReadRegDWORD $2 HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok" "SOUNDMODE"
  ClearErrors
  ReadRegStr $3 HKLM "${HLRO_GRAVITY_SOFT}\RenewSetup" ""
  ${If} ${Errors}
    StrCpy $3 "gone"
  ${EndIf}
  SetRegView default
  !insertmacro EQ "existing uninstall: settings kept" $2 "0"
  !insertmacro EQ "existing uninstall: RenewSetup kept" $3 ""
  ReadRegStr $2 HKCU "${HLRO_GRAVITY}\RagnarokOnline" "RagPath"
  ReadRegStr $3 HKCU "${HLRO_GRAVITY}\RagnarokOnline" "SakrayPath"
  ReadRegStr $4 HKCU "${HLRO_GRAVITY}\RagnarokOnline" ""
  !insertmacro EQ "existing uninstall: RagPath restored" $2 "D:\OtherRO\"
  !insertmacro EQ "existing uninstall: SakrayPath restored" $3 "D:\OtherRO\"
  !insertmacro EQ "existing uninstall: default restored" $4 "D:\OtherRO\"
  SetRegView 32
  DeleteRegKey HKLM "Software\HLROTest"
  SetRegView default
  DeleteRegKey HKLM "Software\HLROTest"
  DeleteRegKey HKCU "Software\HLROTest"

  ${If} $Fails == 0
    FileWrite $Out "ALL PASS$\r$\n"
  ${Else}
    FileWrite $Out "SOME FAILED$\r$\n"
  ${EndIf}
  FileClose $Out
  RMDir /r $9
SectionEnd
