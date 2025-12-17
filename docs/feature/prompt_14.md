1. Run the test and actually look at the impact/numbers. Dont change the code (see below item 4)
2. The 100 limit is a UI reason, rather than performance. i.e. hard to see 500 logs. Are there more efficient UI elements, rather than having DOM elements. i.e. a text box or someting else.
3. THe "Show earlier logs" link has NEVER worked. And exposes the issue of more DOM elements. - Remove.
4. Once all steps are compelted in a job, the page shows ALL log items. Then after a hard reset the page reverts to the 100 limit. This is wrong, however spawned the question.
5. You appear to have reverted the total log update. It's not performing consistantly.