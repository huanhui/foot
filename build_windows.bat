@echo off
echo=

echo copy config
if not exist %cd%\bin\conf\ md %cd%\bin\conf\
xcopy /Y /S %cd%\conf %cd%\bin\conf\
xcopy /Y /S %cd%\conf\app_prod.ini %cd%\bin\conf\app.ini
xcopy /Y /S %cd%\conf\app_pord.conf %cd%\bin\conf\app.conf

echo copy resource
if not exist %cd%\bin\assets\ md %cd%\bin\assets\
xcopy /Y /S %cd%\assets %cd%\bin\assets\

echo build
go build  -v -x  -o bin/FOOT000.exe FOOT000Cmd.go

echo=
pause