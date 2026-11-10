; Install-folder safety for hufflabs-ro.nsi (tested by test-paths.sh).
; !insertmacro HLRO_PATH_FUNCS ""     for the installer
; !insertmacro HLRO_PATH_FUNCS "un."  for the uninstaller

!macro HLRO_PATH_FUNCS un

; Makes $INSTDIR end in a folder called HuffLabsRO, so picking C:\Games installs to
; C:\Games\HuffLabsRO and the uninstaller never has a shared folder to delete.
Function ${un}HlroFixInstDir
  Push $0
  trim:
    StrCpy $0 $INSTDIR 1 -1
    StrCmp $0 "\" 0 +3
      StrCpy $INSTDIR $INSTDIR -1
      Goto trim
  StrCpy $0 $INSTDIR 11 -11
  ${IfNot} $0 == "\HuffLabsRO"   ; case-insensitive
    StrCpy $INSTDIR "$INSTDIR\HuffLabsRO"
  ${EndIf}
  Pop $0
FunctionEnd

; $R0 = 1 if $INSTDIR is missing, empty, or already a HuffLabs RO install
; (finished or half-done), else 0.
Function ${un}HlroTargetOK
  StrCpy $R0 1
  ${If} ${FileExists} "$INSTDIR\*.*"
  ${AndIfNot} ${FileExists} "$INSTDIR\HuffLabsRO.exe"
  ${AndIfNot} ${FileExists} "$INSTDIR\Uninstall.exe"
  ${AndIfNot} ${FileExists} "$INSTDIR\.download\*.*"
  ${AndIfNot} ${FileExists} "$INSTDIR\.unpack\*.*"
  ${AndIfNot} ${FileExists} "$INSTDIR\.hufflabs-launcher\*.*"
    Push $0
    Push $1
    FindFirst $0 $1 "$INSTDIR\*.*"
    ${DoWhile} $1 != ""
      ${If} $1 != "."
      ${AndIf} $1 != ".."
        StrCpy $R0 0
        ${Break}
      ${EndIf}
      FindNext $0 $1
    ${Loop}
    FindClose $0
    Pop $1
    Pop $0
  ${EndIf}
FunctionEnd

; $R0 = 1 if the launcher already keeps $INSTDIR up to date (then setup skips the
; 4 GB download and only refreshes shortcuts and settings).
Function ${un}HlroLauncherInstalled
  StrCpy $R0 0
  ${If} ${FileExists} "$INSTDIR\.hufflabs-launcher\state.json"
    StrCpy $R0 1
  ${EndIf}
FunctionEnd

; Removes the folder on top of the stack and everything in it, but never follows
; junctions or symlinks: a link is removed itself, not what it points to. NSIS's
; RMDir /r follows them (seen under Wine), and the uninstaller runs elevated.
Function ${un}HlroRmTree
  Exch $0
  Push $1
  Push $2
  Push $3
  Push $4
  FindFirst $1 $2 "$0\*.*"
  loop:
    StrCmp $2 "" done
    StrCmp $2 "." next
    StrCmp $2 ".." next
    System::Call 'kernel32::GetFileAttributesW(w "$0\$2") i .r3'
    IntOp $4 $3 & 0x400   ; FILE_ATTRIBUTE_REPARSE_POINT
    ${If} $4 != 0
      IntOp $4 $3 & 0x10
      ${If} $4 != 0
        RMDir "$0\$2"   ; the link itself
      ${Else}
        Delete "$0\$2"
      ${EndIf}
    ${Else}
      IntOp $4 $3 & 0x10   ; FILE_ATTRIBUTE_DIRECTORY
      ${If} $4 != 0
        Push "$0\$2"
        Call ${un}HlroRmTree
      ${Else}
        Delete "$0\$2"
      ${EndIf}
    ${EndIf}
  next:
    FindNext $1 $2
    Goto loop
  done:
  FindClose $1
  RMDir "$0"
  Pop $4
  Pop $3
  Pop $2
  Pop $1
  Pop $0
FunctionEnd

; Deletes the install. Only a folder named HuffLabsRO holding HuffLabsRO.exe is
; wiped whole; anywhere else (an install from an early Setup.exe straight into a
; shared folder) just the uninstaller goes. $R0 = 1 if game files were left behind.
Function ${un}HlroRemoveInstall
  StrCpy $R0 $INSTDIR 11 -11
  ${If} $R0 == "\HuffLabsRO"
  ${AndIf} ${FileExists} "$INSTDIR\HuffLabsRO.exe"
    SetOutPath "$TEMP"   ; can't delete the current folder
    Push "$INSTDIR"
    Call ${un}HlroRmTree
    StrCpy $R0 0
  ${Else}
    Delete "$INSTDIR\Uninstall.exe"
    StrCpy $R0 1
  ${EndIf}
FunctionEnd

!macroend
