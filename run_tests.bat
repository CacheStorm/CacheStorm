@echo off
echo ============================================================
echo              CacheStorm Test Suite Runner
echo ============================================================
echo.

echo Running all tests...
go test ./internal/... -v -count=1 > test_output.txt 2>&1

echo.
echo ============================================================
echo                    TEST RESULTS
echo ============================================================

findstr /C:"PASS" /C:"FAIL" test_output.txt | findstr /C:"---"
echo.

echo ============================================================
echo                    SUMMARY
echo ============================================================

for /f %%i in ('findstr /C:"--- PASS" test_output.txt ^| find /c /v ""') do set PASSED=%%i
for /f %%i in ('findstr /C:"--- FAIL" test_output.txt ^| find /c /v ""') do set FAILED=%%i

echo Tests Passed: %PASSED%
echo Tests Failed: %FAILED%
echo.

if %FAILED% EQU 0 (
    echo [SUCCESS] All tests passed! 100%% success rate.
) else (
    echo [FAILURE] Some tests failed.
)

echo.
echo ============================================================
echo                  COVERAGE REPORT
echo ============================================================
go test ./internal/... -coverprofile=coverage.out >nul 2>&1
go tool cover -func=coverage.out | findstr "total"

del test_output.txt coverage.out 2>nul

echo.
pause
