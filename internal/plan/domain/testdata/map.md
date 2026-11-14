# Map: Invoice export

## Destination

Customers download a monthly invoice export as a CSV file.
The map holds the decisions before the plan.

## Notes

The map plans and does not do. A ticket ends with an answer.

## Decisions so far

- T1: the export is a CSV file with one row for each invoice line.
- T2: the export runs as a job and not as a request.

## Not yet specified

- The retention of old exports.
- The access rule for an export of another team.

## Out of scope

- none

## Tickets

### T1 file-format

- Type: grilling
- Blocked by: none
- Status: closed
- Question: Which file format does the export use?
- Answer: A CSV file with one row for each invoice line.

### T2 run-mode

- Type: research
- Blocked by: T1
- Status: closed
- Question: Does the export run as a request or as a job?
- Answer: As a job, because a large account takes longer than a request limit.

### T3 job-queue

- Type: prototype
- Blocked by: T1 and T2
- Status: open
- Question: Does the existing queue carry the export job?

### T4 retry-rule

- Type: grilling
- Blocked by: T3
- Status: open
- Question: How many times does a failed export job retry?

### T5 file-store

- Type: research
- Blocked by: none
- Status: open
- Claimed by: ana
- Question: Where does the store keep a finished export file?

### T6 download-link

- Type: grilling
- Blocked by: T1, T3 and T5
- Status: open
- Question: How long does a download link stay valid?

### T7 bucket-access

- Type: work
- Blocked by: T2
- Status: open
- Question: Who files the access request for the bucket?
