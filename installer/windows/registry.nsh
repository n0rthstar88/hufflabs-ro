; Game registry keys for hufflabs-ro.nsi (tested by test-nsis.sh).
; !insertmacro HLRO_REG_FUNCS ""     for the installer
; !insertmacro HLRO_REG_FUNCS "un."  for the uninstaller
;
; The Gravity keys are shared with any other Ragnarok client on the PC. So the
; installer only creates the settings keys when they're missing, remembers what
; it created or replaced (in the uninstall key), and the uninstaller removes only
; what it created and puts back the paths it replaced.

!ifndef HLRO_GRAVITY_SOFT
  !define HLRO_GRAVITY_SOFT "Software\Gravity Soft"
!endif
!ifndef HLRO_GRAVITY
  !define HLRO_GRAVITY "Software\Gravity"
!endif
!ifndef HLRO_STATE_KEY
  !define HLRO_STATE_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\HuffLabsRO"
!endif

!macro HLRO_REG_FUNCS un

Function ${un}HlroWriteGameKeys
  Push $0
  Push $1
  Push $2
  Push $3
  ; $1/$2 = 1 when this run creates the settings / RenewSetup key
  StrCpy $1 0
  StrCpy $2 0
  SetRegView 32
  ClearErrors
  EnumRegValue $0 HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok" 0
  ${If} $0 == ""
    ; Same values as client/defaults/settings.reg (keep in sync).
    WriteRegDWORD HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok" "BITWISE" 1
    WriteRegDWORD HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok" "CHECKSAVE" 0
    WriteRegStr   HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok" "DEVICENAME" "\\.\DISPLAY1"
    WriteRegDWORD HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok" "DIGITALBITSTYPE" 0
    WriteRegDWORD HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok" "DIGITALRATETYPE" 0
    WriteRegBin   HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok" "GUIDDEVICE" 00000000000000000000000000000000
    WriteRegBin   HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok" "GUIDDRIVER" 00000000650000000000000000000000
    WriteRegDWORD HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok" "NUMACCOUNT" 0
    WriteRegDWORD HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok" "NUMSERVER" 0
    WriteRegStr   HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok" "PROVIDERNAME" "No Provider"
    WriteRegDWORD HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok" "SHOWTIPSATSTARTUP" 1
    WriteRegDWORD HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok" "SOUNDMODE" 1
    WriteRegDWORD HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok" "SPEAKERTYPE" 0
    StrCpy $1 1
  ${EndIf}
  ; The client waits forever at startup without the RenewSetup key.
  ClearErrors
  ReadRegStr $0 HKLM "${HLRO_GRAVITY_SOFT}\RenewSetup" ""
  ${If} ${Errors}
    WriteRegStr HKLM "${HLRO_GRAVITY_SOFT}\RenewSetup" "" ""
    StrCpy $2 1
  ${EndIf}
  SetRegView default

  ; Record ownership once, on the first install; a reinstall finds the keys it
  ; made itself and must not forget it made them.
  ClearErrors
  ReadRegDWORD $0 HKLM "${HLRO_STATE_KEY}" "GravityRecorded"
  ${If} ${Errors}
    WriteRegDWORD HKLM "${HLRO_STATE_KEY}" "OwnsSettingsKey" $1
    WriteRegDWORD HKLM "${HLRO_STATE_KEY}" "OwnsRenewSetup" $2
    ClearErrors
    ReadRegStr $3 HKCU "${HLRO_GRAVITY}\RagnarokOnline" "RagPath"
    ${If} ${Errors}
      WriteRegDWORD HKLM "${HLRO_STATE_KEY}" "OwnsPathKey" 1
    ${Else}
      WriteRegDWORD HKLM "${HLRO_STATE_KEY}" "OwnsPathKey" 0
      WriteRegStr HKLM "${HLRO_STATE_KEY}" "PrevRagPath" $3
      ReadRegStr $3 HKCU "${HLRO_GRAVITY}\RagnarokOnline" "SakrayPath"
      WriteRegStr HKLM "${HLRO_STATE_KEY}" "PrevSakrayPath" $3
      ReadRegStr $3 HKCU "${HLRO_GRAVITY}\RagnarokOnline" ""
      WriteRegStr HKLM "${HLRO_STATE_KEY}" "PrevDefaultPath" $3
    ${EndIf}
    WriteRegDWORD HKLM "${HLRO_STATE_KEY}" "GravityRecorded" 1
  ${EndIf}

  ; The client finds its folder through these, so they point here while installed.
  WriteRegStr HKCU "${HLRO_GRAVITY}\RagnarokOnline" "" "$INSTDIR\"
  WriteRegStr HKCU "${HLRO_GRAVITY}\RagnarokOnline" "RagPath" "$INSTDIR\"
  WriteRegStr HKCU "${HLRO_GRAVITY}\RagnarokOnline" "SakrayPath" "$INSTDIR\"
  Pop $3
  Pop $2
  Pop $1
  Pop $0
FunctionEnd

Function ${un}HlroRemoveGameKeys
  Push $0
  ReadRegDWORD $0 HKLM "${HLRO_STATE_KEY}" "OwnsSettingsKey"
  SetRegView 32
  ${If} $0 == 1
    DeleteRegKey HKLM "${HLRO_GRAVITY_SOFT}\Ragnarok"
  ${EndIf}
  SetRegView default
  ReadRegDWORD $0 HKLM "${HLRO_STATE_KEY}" "OwnsRenewSetup"
  SetRegView 32
  ${If} $0 == 1
    DeleteRegKey HKLM "${HLRO_GRAVITY_SOFT}\RenewSetup"
  ${EndIf}
  SetRegView default
  ReadRegDWORD $0 HKLM "${HLRO_STATE_KEY}" "OwnsPathKey"
  ${If} $0 == 0
    ReadRegStr $0 HKLM "${HLRO_STATE_KEY}" "PrevRagPath"
    WriteRegStr HKCU "${HLRO_GRAVITY}\RagnarokOnline" "RagPath" $0
    ReadRegStr $0 HKLM "${HLRO_STATE_KEY}" "PrevSakrayPath"
    WriteRegStr HKCU "${HLRO_GRAVITY}\RagnarokOnline" "SakrayPath" $0
    ReadRegStr $0 HKLM "${HLRO_STATE_KEY}" "PrevDefaultPath"
    WriteRegStr HKCU "${HLRO_GRAVITY}\RagnarokOnline" "" $0
  ${Else}
    ; also the case for installs from before this was recorded
    DeleteRegKey HKCU "${HLRO_GRAVITY}\RagnarokOnline"
  ${EndIf}
  Pop $0
FunctionEnd

!macroend
