@echo off
REM OnDex dev stekini istalgan papkadan ishga tushirish uchun shim.
REM
REM Bu fayl PATH ga qo'shilgan (F:\ChustApp\scripts), shuning uchun
REM `ondex run` buyrug'i cmd'da ham, PowerShell'da ham ishlaydi.
REM
REM `-NoProfile` ATAYLAB: foydalanuvchi profilidagi sekin import'lar va
REM alias'lar skript xatti-harakatini o'zgartirmasligi uchun.
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0ondex.ps1" %*
