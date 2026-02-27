@echo off
:: ------------------------------
:: Script: psmp
:: Autore: Giuseppe Criaco
:: 23/01/2026   
:: ------------------------------

echo === Impostazione permessi su chiave privata ===
Set KEY="c:\temp\key.pem"
icacls %KEY% /inheritance:r
icacls %KEY% /grant:r %USERNAME%:F
icacls %KEY%

echo === Permessi impostati ===
echo.

:INIT
set /p HOST=IP o nome DNS: 
ssh %USERNAME%@root@%HOST%@psmp.sogei.it -i %KEY%

goto INIT