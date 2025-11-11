
REM ================================
REM CLI Agent Template
REM Available environment variables:
REM   $env:TRAYCER_PROMPT - The prompt to be executed (environment variable set by Traycer at runtime)
REM   $env:TRAYCER_TASK_ID - Traycer task identifier - use this when you want to use the same session on the execution agent across phase iterations, plans, and verification execution
REM   $env:TRAYCER_PHASE_BREAKDOWN_ID - Traycer phase breakdown identifier - use this when you want to use the same session for the current list of phases
REM   $env:TRAYCER_PHASE_ID - Traycer per phase identifier - use this when you want to use the same session for plan/review and verification
REM
REM NOTE: This template uses PowerShell syntax ($env:) by default.
REM
REM For other terminals, clone this template and modify as follows:
REM   PowerShell:   $env:TRAYCER_PROMPT, $env:TRAYCER_TASK_ID, $env:TRAYCER_PHASE_BREAKDOWN_ID, $env:TRAYCER_PHASE_ID
REM   Git Bash: $TRAYCER_PROMPT, $TRAYCER_TASK_ID, $TRAYCER_PHASE_BREAKDOWN_ID, $TRAYCER_PHASE_ID
REM
REM CMD is not supported at the moment.
REM ================================

echo claude --dangerously-skip-permissions "$env:TRAYCER_PROMPT"
