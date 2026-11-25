# GenAI Tools – assistance record

Course: MSDS 431 — Data Engineering with Go
Project: Personal Movie Collection Manager
Repository: https://github.com/CHbobray/go-duckdb-movie-manager.git
Tools: ChatGPT, Claude

## How I used ChatGPT

I provided the assignment requirements and four IMDb CSV files. I used ChatGPT to help inspect the data, design the relational schema, write the Python/Polars loader and Go command-line application, develop automated tests, and draft the README. ChatGPT also calculated reference genre results from the source data before I ran the application.

## Development and troubleshooting summary

| Stage | Assistance provided | My local verification |
|---|---|---|
| Setup | Identified incorrect PowerShell working directory and nested project folder | Installed Python dependencies successfully |
| Data loading | Drafted loader and schema | Loaded 363,079 records in 13.945 seconds |
| Go compilation | Diagnosed disabled CGO and missing GCC | Installed MSYS2 and enabled CGO |
| Linker error | Suggested WinLibs GCC 14.2.0 after GCC 16.2.0 failed; corrected compiler path | Both original tests passed |
| Application | Suggested manual checks for menu functions and invalid inputs | Completed all 12 planned checks successfully; built and launched executable |
| Automated tests | Expanded the suite from two to seven test functions using synthetic in-memory data | All tests passed; statement coverage increased from 4.0% to 85.1% |
| Coverage report | Corrected PowerShell quoting for coverage arguments | Generated function-level and HTML coverage reports |
| Rubric review (Claude) | Fixed truncated actors/roles CSVs in the repository; refactored the menu into one function per option; added comments, a missing-movie message, a simpler rating check and two benchmarks | Nine tests passed; coverage 86.2% |
| Documentation | Helped interpret results and update README with actual measurements and repository URL | Results were taken from my local execution screenshots |

## Limitations and verification

The initial AI-generated project could not be run end to end in the drafting environment. I installed the dependencies and executed the programs on my Windows computer. I provided screenshots throughout troubleshooting and testing. The final coverage report shows 86.2% overall: 100% for most functions, 90.0% for printQuery, 92.3% for saveCollectionItem, and 0% for main, which was verified manually instead. The reported memory measurements do not represent total process memory. Optional performance comparisons were not performed.

## Conversation documentation

This file summarizes the interaction; it is not a verbatim conversation transcript. The full ChatGPT conversation is available at this share link:

https://chatgpt.com/share/6b063870-146c-83ea-8cb4-c4f7489e9039

The conversation was reviewed for personal information before sharing.
