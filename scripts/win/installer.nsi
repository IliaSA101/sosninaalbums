; Установщик Sosnina Albums (спека 111): на пользователя, без прав администратора и без выбора папки.
; Сборка — scripts/win/installer.sh (makensis 3.08+, -WX): -DVERSION=X.Y.Z -DEXE_SRC=<exe> -DICON=<ico> -DWIZARD=<bmp> -DWIZARD_DPI=<папка с wizard-100/125/150/175.bmp> -DHEADER=<bmp> -DOUT=<файл> [-DDEMO_SRC=<zip примера>].
; -DDEMO_SRC — стандартная редакция (0.30-142-44): zip примера кладётся в $INSTDIR\data\; без него — компактная.
; Коды выхода (контракт с installer-check.ps1 и программой): 0 готово; 1 отмена; 2 ошибка NSIS;
; 3 откат отклонён (установлена новее); 4 та же версия (тихо); 5 программа не закрылась;
; 6 не удалось заменить exe (занят антивирусом; прежняя программа и версия в реестре целы); 64 система не та.
; Параметры: /S тихо, /D=<папка> (без кавычек, последним), /relaunch — открыть программу по окончании;
; при отказе (3, 4, сбой) /relaunch открывает прежний exe. Других параметров установщик не читает.
Unicode true
; Время файлов в установщик не пишем нигде (ни exe, ни картинки MUI2 в .onInit/un.onInit): иначе байты зависят от mtime
; на сборщике (CI и пересборка release-sign.mjs берут файлы в разное время) и побайтная сверка не сходится
SetDateSave off
SetCompressor /SOLID lzma
RequestExecutionLevel user
ManifestDPIAware true

!ifndef VERSION
  !error "нужен -DVERSION=X.Y.Z"
!endif
!ifndef EXE_SRC
  !error "нужен -DEXE_SRC=путь к exe"
!endif
!ifndef ICON
  !error "нужен -DICON=путь к icon.ico"
!endif
!ifndef WIZARD
  !error "нужен -DWIZARD=путь к wizard.bmp (328x628 = 164x314 в 2x, make_wizard_bmp.py)"
!endif
!ifndef WIZARD_DPI
  !error "нужен -DWIZARD_DPI=папка с wizard-100.bmp, wizard-125.bmp, wizard-150.bmp, wizard-175.bmp (make_wizard_bmp.py)"
!endif
!ifndef HEADER
  !error "нужен -DHEADER=путь к header.bmp (114x114, make_wizard_bmp.py)"
!endif
!ifndef OUT
  !define OUT "SosninaAlbums-${VERSION}-windows-x64-setup.exe"
!endif

!define PRODUCT "Sosnina Albums"
!define EXE "SosninaAlbums.exe"
!define REGKEY "Software\SosninaAlbums"
!define UNINST "Software\Microsoft\Windows\CurrentVersion\Uninstall\SosninaAlbums"
!define PROGID "SosninaAlbums.Album"
; Замок второго экземпляра Wails (single_instance.go: "wails-app-" + UniqueId + "sim"); UniqueId — app.instanceID
!define MUTEX "wails-app-6f1c2b7e-3a4d-4e58-9b0a-5d2e8c7f1a93sim"

!include "MUI2.nsh"
!include "FileFunc.nsh"
!include "WordFunc.nsh"
!include "LogicLib.nsh"
!include "WinVer.nsh"
!include "x64.nsh"

Name "${PRODUCT} ${VERSION}"
OutFile "${OUT}"
InstallDir "$LOCALAPPDATA\Programs\${PRODUCT}"
InstallDirRegKey HKCU "${REGKEY}" "InstallDir"
BrandingText "${PRODUCT} ${VERSION}"

VIProductVersion "${VERSION}.0"
VIFileVersion "${VERSION}.0"
VIAddVersionKey /LANG=1049 "ProductName" "${PRODUCT}"
VIAddVersionKey /LANG=1049 "FileDescription" "Установка ${PRODUCT}"
VIAddVersionKey /LANG=1049 "ProductVersion" "${VERSION}"
VIAddVersionKey /LANG=1049 "FileVersion" "${VERSION}"
VIAddVersionKey /LANG=1049 "LegalCopyright" "© 2026 Илья Соснин. GPL-3.0-only"

!define MUI_ICON "${ICON}"
!define MUI_UNICON "${ICON}"
!define MUI_ABORTWARNING
; Картинки — свои (scripts/win/make_wizard_bmp.py). WIZARD — 2x, для 200 % и выше: при масштабе NSIS сжимает её
; в контрол без сглаживания. На 100-175 % WizardForDpi подменяет её картинкой точно под высоту контрола,
; тогда AspectFitHeight ничего не тянет. Он же держит пропорции при другой ширине контрола (FitControl
; по умолчанию искажал бы их). Слева на первой странице и на странице завершения — WIZARD, справа в шапке
; внутренних страниц вместо значка установщика — HEADER (значок 32 px при масштабе размывался)
!define MUI_WELCOMEFINISHPAGE_BITMAP "${WIZARD}"
!define MUI_WELCOMEFINISHPAGE_BITMAP_STRETCH AspectFitHeight
; Вызывается из .onGUIInit после того, как MUI2 положил $PLUGINSDIR\modern-wizard.bmp, и до создания первой страницы
!define MUI_CUSTOMFUNCTION_GUIINIT WizardForDpi
!define MUI_HEADERIMAGE
!define MUI_HEADERIMAGE_RIGHT
!define MUI_HEADERIMAGE_BITMAP "${HEADER}"
!define MUI_HEADERIMAGE_BITMAP_STRETCH AspectFitHeight
; Текст первой страницы — переменная: при повторном запуске той же версии .onInit подменяет его
Var WelcomeText
!define MUI_WELCOMEPAGE_TITLE "${PRODUCT} ${VERSION}"
!define MUI_WELCOMEPAGE_TEXT "$WelcomeText"
!define MUI_FINISHPAGE_RUN "$INSTDIR\${EXE}"
; Стандартные русские тексты NSIS пишут «Готово» и «Удалить» в прямых кавычках: свои, в «ёлочках»
!define MUI_FINISHPAGE_TEXT "Установка $(^NameDA) выполнена.$\r$\n$\r$\nНажмите «Готово» для выхода из программы установки."
!define MUI_UNCONFIRMPAGE_TEXT_TOP "Программа $(^NameDA) будет удалена с этого компьютера. Настройки и альбомы останутся.$\r$\n$\r$\nНажмите «Удалить», чтобы удалить программу."
; Страницы выбора папки нет (решение 0.25-q111-pair): папка постоянная, лишнего вопроса нет;
; ставим и удаляем только поимённо — даже /D= на «Рабочий стол» ничего чужого не тронет
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "Russian"

Var Relaunch
Var FailCode

; Картинка мастера под масштаб экрана: система DPI-aware (ManifestDPIAware), высота контрола картинки на
; 100/125/150/175 % — 314/386/458/555 px (измерено на снимках CI). Пороги — 96/120/144/168 dpi; промежуточный
; масштаб берёт ближайшую бо́льшую картинку, 200 % и выше остаются с 2x от MUI2. Не вышло прочитать DPI — 2x.
Function WizardForDpi
  Push $0
  Push $1
  System::Call 'user32::GetDC(p 0) p .r0'
  ${If} $0 P<> 0
    System::Call 'gdi32::GetDeviceCaps(p r0, i 90) i .r1'
    System::Call 'user32::ReleaseDC(p 0, p r0)'
    ${If} $1 > 0
      ${If} $1 <= 96
        File "/oname=$PLUGINSDIR\modern-wizard.bmp" "${WIZARD_DPI}/wizard-100.bmp"
        !pragma verifyloadimage "${WIZARD_DPI}/wizard-100.bmp"
      ${ElseIf} $1 <= 120
        File "/oname=$PLUGINSDIR\modern-wizard.bmp" "${WIZARD_DPI}/wizard-125.bmp"
        !pragma verifyloadimage "${WIZARD_DPI}/wizard-125.bmp"
      ${ElseIf} $1 <= 144
        File "/oname=$PLUGINSDIR\modern-wizard.bmp" "${WIZARD_DPI}/wizard-150.bmp"
        !pragma verifyloadimage "${WIZARD_DPI}/wizard-150.bmp"
      ${ElseIf} $1 <= 168
        File "/oname=$PLUGINSDIR\modern-wizard.bmp" "${WIZARD_DPI}/wizard-175.bmp"
        !pragma verifyloadimage "${WIZARD_DPI}/wizard-175.bmp"
      ${EndIf}
    ${EndIf}
  ${EndIf}
  Pop $1
  Pop $0
FunctionEnd

; Тихое обновление (/relaunch) не поставилось — открыть прежнюю программу: она увидит, что
; версия не та, что ждали (pending), и скажет «Обновление не установилось»
Function RelaunchOld
  ${If} $Relaunch == 1
    IfFileExists "$INSTDIR\${EXE}" 0 +2
      Exec '"$INSTDIR\${EXE}"'
  ${EndIf}
FunctionEnd

Function .onInstFailed
  ; Abort мог сбросить код выхода в 2 — возвращаем свой, если он был назначен
  ${If} $FailCode != ""
    SetErrorLevel $FailCode
  ${EndIf}
  Call RelaunchOld
FunctionEnd

Function .onInit
  SetRegView 64
  StrCpy $WelcomeText "Будет установлена программа вёрстки свадебных фотоальбомов ${PRODUCT} ${VERSION}.$\r$\n$\r$\nПапка установки: %LOCALAPPDATA%\Programs\${PRODUCT}. Права администратора не нужны.$\r$\n$\r$\nЕсли программа открыта, закройте её."
  ${IfNot} ${AtLeastWin10}
  ${OrIfNot} ${RunningX64}
    MessageBox MB_OK|MB_ICONSTOP "Нужна 64-разрядная Windows 10 или 11." /SD IDOK
    SetErrorLevel 64
    Quit
  ${EndIf}
  ${GetParameters} $R0
  ClearErrors
  ${GetOptions} $R0 "/relaunch" $R1
  ${IfNot} ${Errors}
    StrCpy $Relaunch 1
  ${EndIf}
  ReadRegStr $R2 HKCU "${UNINST}" "DisplayVersion"
  ${If} $R2 != ""
    ${VersionCompare} "$R2" "${VERSION}" $R3
    ${If} $R3 == 1
      IfSilent 0 +4
        SetErrorLevel 3
        Call RelaunchOld
        Quit
      MessageBox MB_YESNO|MB_ICONEXCLAMATION|MB_DEFBUTTON2 "Заменить более новую $R2 версией ${VERSION}?" IDYES +3
      SetErrorLevel 3
      Quit
    ${ElseIf} $R3 == 0
      IfSilent 0 +4
        SetErrorLevel 4
        Call RelaunchOld
        Quit
      StrCpy $WelcomeText "${PRODUCT} ${VERSION} уже установлена.$\r$\n$\r$\n«Установить» — переустановит её (настройки и альбомы не тронутся).$\r$\n$\r$\nЕсли программа открыта, закройте её."
    ${EndIf}
  ${EndIf}
FunctionEnd

; Ждём, пока программа закроется: замок Wails (любая копия, в том числе из zip) и сам exe —
; запущенный exe Windows переписать не даёт. Тихое обновление ждёт до 60 с (программа сохраняет
; альбом и выходит), обычная установка сразу спрашивает.
Function WaitUnlocked
  StrCpy $R5 0
  ; Мастер после «Обновить» в переносной копии стартует, пока программа ещё выходит: ждём 5 с
  StrCpy $R8 10
  IfSilent 0 +2
    StrCpy $R8 120
  loop:
    System::Call 'kernel32::OpenMutexW(i 0x00100000, i 0, w "${MUTEX}") p .R7'
    ${If} $R7 P<> 0
      System::Call 'kernel32::CloseHandle(p R7)'
      Goto busy
    ${EndIf}
    IfFileExists "$INSTDIR\${EXE}" 0 done
    ClearErrors
    FileOpen $R6 "$INSTDIR\${EXE}" a
    ${IfNot} ${Errors}
      FileClose $R6
      Goto done
    ${EndIf}
  busy:
    IntOp $R5 $R5 + 1
    ${If} $R5 < $R8
      Sleep 500
      Goto loop
    ${EndIf}
    MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "Закройте Sosnina Albums и нажмите «Повтор»." /SD IDCANCEL IDRETRY retry
    SetErrorLevel 5
    Quit
  retry:
    StrCpy $R5 0
    Goto loop
  done:
FunctionEnd

; Замена exe: старый уходит в .old, новый (.new) встаёт на его место. Занятый антивирусом файл
; повторяем 5 с; не вышло — откат (.old возвращается), код 6, Abort: реестр, ярлыки и версия
; не тронуты, .onInstFailed откроет прежнюю программу.
Function ReplaceExe
  Push $R0
  Push $R1
  Delete "$INSTDIR\${EXE}.old"
  StrCpy $R1 0
  StrCpy $R0 0
  ; шаг 1: старый exe -> .old (первой установки нет — шаг пропускаем)
  moveold:
    ClearErrors
    IfFileExists "$INSTDIR\${EXE}" 0 placenew
    Rename "$INSTDIR\${EXE}" "$INSTDIR\${EXE}.old"
    ${IfNot} ${Errors}
      StrCpy $R1 1
      StrCpy $R0 0
      Goto placenew
    ${EndIf}
    IntOp $R0 $R0 + 1
    ${If} $R0 < 10
      Sleep 500
      Goto moveold
    ${EndIf}
    Goto fail
  ; шаг 2: .new -> exe
  placenew:
    ClearErrors
    Rename "$INSTDIR\${EXE}.new" "$INSTDIR\${EXE}"
    ${IfNot} ${Errors}
      Goto done
    ${EndIf}
    IntOp $R0 $R0 + 1
    ${If} $R0 < 10
      Sleep 500
      Goto placenew
    ${EndIf}
    ${If} $R1 == 1
      StrCpy $R0 0
      back:
        ClearErrors
        Rename "$INSTDIR\${EXE}.old" "$INSTDIR\${EXE}"
        ${IfNot} ${Errors}
          Goto fail
        ${EndIf}
        IntOp $R0 $R0 + 1
        ${If} $R0 < 10
          Sleep 500
          Goto back
        ${EndIf}
    ${EndIf}
  fail:
    Delete "$INSTDIR\${EXE}.new"
    StrCpy $FailCode 6
    SetErrorLevel 6
    MessageBox MB_OK|MB_ICONSTOP "Не удалось заменить программу: файл занят (возможно, антивирусом). Прежняя версия осталась на месте. Повторите установку." /SD IDOK
    Abort
  done:
  Pop $R1
  Pop $R0
FunctionEnd

Section "main"
  SetShellVarContext current
  Call WaitUnlocked
  SetOutPath "$INSTDIR"
  ; Новая копия рядом, затем замена (ReplaceExe): сбой не оставляет ни битый, ни пропавший exe;
  ; реестр пишется только после успешной замены
  Delete "$INSTDIR\${EXE}.new"
  File "/oname=${EXE}.new" "${EXE_SRC}"
  Call ReplaceExe
!ifdef DEMO_SRC
  ; Стандартная редакция: пример свадебного макета рядом с программой (0.30-142-44). Блок после ReplaceExe:
  ; сбой замены exe не оставит пример без программы. Имя файла — как у demopack.File (оно же в маске удаления)
  SetOutPath "$INSTDIR\data"
  File "${DEMO_SRC}"
  ; Рабочую папку наследуют CreateShortcut и Exec ниже — возвращаем её
  SetOutPath "$INSTDIR"
!endif
  WriteUninstaller "$INSTDIR\uninstall.exe"

  WriteRegStr HKCU "${REGKEY}" "InstallDir" "$INSTDIR"
  WriteRegStr HKCU "${REGKEY}" "Version" "${VERSION}"
  WriteRegStr HKCU "${UNINST}" "DisplayName" "${PRODUCT}"
  WriteRegStr HKCU "${UNINST}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINST}" "Publisher" "Илья Соснин"
  WriteRegStr HKCU "${UNINST}" "DisplayIcon" "$INSTDIR\${EXE},0"
  WriteRegStr HKCU "${UNINST}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINST}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr HKCU "${UNINST}" "QuietUninstallString" '"$INSTDIR\uninstall.exe" /S'
  WriteRegStr HKCU "${UNINST}" "URLInfoAbout" "https://github.com/IliaSA101/sosninaalbums"
  WriteRegDWORD HKCU "${UNINST}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINST}" "NoRepair" 1
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  WriteRegDWORD HKCU "${UNINST}" "EstimatedSize" $0

  ; Ярлыки: при тихом обновлении удалённый пользователем ярлык на столе не возвращаем
  CreateShortcut "$SMPROGRAMS\${PRODUCT}.lnk" "$INSTDIR\${EXE}"
  ${If} ${Silent}
    IfFileExists "$DESKTOP\${PRODUCT}.lnk" 0 +2
      CreateShortcut "$DESKTOP\${PRODUCT}.lnk" "$INSTDIR\${EXE}"
  ${Else}
    CreateShortcut "$DESKTOP\${PRODUCT}.lnk" "$INSTDIR\${EXE}"
  ${EndIf}

  ; .salbum за текущим пользователем (подробно — спека 112). Связь пишем, только если расширение
  ; свободно или уже наше и пользователь её не выключал (AssocOff — SetAssoc(false)): тихое
  ; обновление не должно снимать его выбор и перетирать чужой ProgID (его assoc.plan в internal/app/assoc не трогает)
  ReadRegStr $R0 HKCU "Software\Classes\.salbum" ""
  ReadRegStr $R1 HKCU "${REGKEY}" "AssocOff"
  ${If} $R1 != 1
  ${AndIf} $R0 == ""
    StrCpy $R2 1
  ${ElseIf} $R1 != 1
  ${AndIf} $R0 == "${PROGID}"
    StrCpy $R2 1
  ${Else}
    StrCpy $R2 0
  ${EndIf}
  ${If} $R2 == 1
    WriteRegStr HKCU "Software\Classes\.salbum" "" "${PROGID}"
    ; NSIS не умеет REG_NONE, поэтому значение REG_SZ; для Windows разницы нет
    WriteRegStr HKCU "Software\Classes\.salbum\OpenWithProgids" "${PROGID}" ""
    WriteRegStr HKCU "Software\Classes\${PROGID}" "" "Альбом Sosnina Albums"
    WriteRegStr HKCU "Software\Classes\${PROGID}" "AppVersion" "${VERSION}"
    WriteRegStr HKCU "Software\Classes\${PROGID}\DefaultIcon" "" '"$INSTDIR\${EXE}",0'
    WriteRegStr HKCU "Software\Classes\${PROGID}\shell\open\command" "" '"$INSTDIR\${EXE}" "%1"'
  ${EndIf}
  System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, p 0, p 0)'
  Delete "$INSTDIR\${EXE}.old"

  ${If} $Relaunch == 1
    Exec '"$INSTDIR\${EXE}"'
  ${EndIf}
SectionEnd

Section "Uninstall"
  SetShellVarContext current
  SetRegView 64
  ; Настройки, шаблоны, библиотека, миниатюры и копии остаются; скачанные обновления — мусор
  ; Единственное RMDir /r — по постоянному пути папки данных, не по $INSTDIR
  RMDir /r "$LOCALAPPDATA\SosninaAlbums\Updates"
  Delete "$INSTDIR\${EXE}"
  Delete "$INSTDIR\${EXE}.new"
  Delete "$INSTDIR\${EXE}.old"
  Delete "$INSTDIR\uninstall.exe"
  ; Пример (только стандартная редакция): маской по префиксу — деинсталлятор компактной собирается без DEMO_SRC,
  ; а v2 примера не оставит сиротой v1. RMDir без /r: чужое в data\ не трогаем; оба до RMDir "$INSTDIR"
  Delete "$INSTDIR\data\wedding-navy-watercolor-demo-*.zip"
  RMDir "$INSTDIR\data"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\${PRODUCT}.lnk"
  Delete "$DESKTOP\${PRODUCT}.lnk"
  ; Ассоциацию снимаем, только если она ведёт в эту папку: её могла забрать копия из zip (112)
  ReadRegStr $R1 HKCU "Software\Classes\${PROGID}\shell\open\command" ""
  ${If} $R1 == '"$INSTDIR\${EXE}" "%1"'
    ReadRegStr $R0 HKCU "Software\Classes\.salbum" ""
    ${If} $R0 == "${PROGID}"
      DeleteRegValue HKCU "Software\Classes\.salbum" ""
    ${EndIf}
    DeleteRegValue HKCU "Software\Classes\.salbum\OpenWithProgids" "${PROGID}"
    DeleteRegKey HKCU "Software\Classes\${PROGID}"
    ; Пустые ключи не оставляем (сначала вложенный); чужие значения держат их на месте
    DeleteRegKey /ifempty HKCU "Software\Classes\.salbum\OpenWithProgids"
    DeleteRegKey /ifempty HKCU "Software\Classes\.salbum"
  ${EndIf}
  DeleteRegKey HKCU "${UNINST}"
  DeleteRegKey HKCU "${REGKEY}"
  System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, p 0, p 0)'
SectionEnd
