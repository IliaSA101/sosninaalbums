# Запекает оглавление и номера страниц в .docx через Word COM и экспортирует PDF
# (кликабельное оглавление, внешние ссылки, закладки по заголовкам).
# Вызывается из build_docx.py --pdf на Windows; нужен установленный Word, файл не открыт.
# Нет Word или сбой — вывод "ERROR: …", PDF не создаётся, build_docx.py пробует LibreOffice.
#
#   powershell -File bake_toc.ps1 -Path "…\x.docx" -Pdf "…\x.pdf"

param([string]$Path, [string]$Pdf)

if (-not $Path -or -not (Test-Path $Path)) { throw "DOCX не найден: $Path" }

$word = $null
try {
  $word = New-Object -ComObject Word.Application
  $word.Visible = $false
  $word.DisplayAlerts = 0  # wdAlertsNone
  $doc = $word.Documents.Open((Resolve-Path $Path).Path, $false, $false)
  foreach ($toc in $doc.TablesOfContents) { $toc.Update() }
  $null = $doc.Fields.Update()
  foreach ($sec in $doc.Sections) {
    foreach ($hf in @($sec.Headers; $sec.Footers)) {
      $null = $hf.Range.Fields.Update()
    }
  }
  $doc.Save()
  Write-Output "TOC baked: $Path"
  if ($Pdf) {
    # 17 = wdExportFormatPDF; CreateBookmarks=1; DocStructureTags=true;
    # UseISO19005_1=false: режим PDF/A убивает живые гиперссылки.
    $doc.ExportAsFixedFormat($Pdf, 17, $false, 0, 0, 0, 0, 0, $false, $true,
      1, $true, $true, $false)
    Write-Output "PDF exported: $Pdf"
  }
  $doc.Close($true)
} catch {
  Write-Output "ERROR: $($_.Exception.Message)"
} finally {
  if ($word) {
    $word.Quit()
    [System.Runtime.InteropServices.Marshal]::ReleaseComObject($word) | Out-Null
  }
}
