# Nano-Guard: Prompt Engineering

## 1. Design Goals for a 3B Model

Small local models (2B–4B parameters) are prone to:
- Hallucinating issues that don't exist
- Ignoring the output format instruction
- Being overly verbose instead of returning pure JSON

To counter this, the system prompt must be:
1. **Short** (< 200 tokens)
2. **Imperative** (commands, not suggestions)
3. **Schema-first** (show the JSON shape before describing rules)
4. **Zero-tolerance on format** ("output ONLY JSON")

---

## 2. System Prompt

> **File**: `prompts/system.txt` (loaded at runtime, not hardcoded)

```
You are a strict code reviewer. Output ONLY valid JSON. No explanation. No prose.

JSON schema:
{
  "approved": boolean,
  "errors": [string],
  "warnings": [string],
  "summary": string
}

Rules — set approved=false ONLY if any rule below is violated:
1. UNHANDLED_ERROR: function returns error/promise but caller ignores it
2. DEBUG_LOG: console.log / fmt.Println / print() left in code
3. TYPE_UNSAFE: use of `any`, unsafe casts, or missing types on public API
4. PLACEHOLDER: TODO, FIXME, panic("implement me"), empty stub body

Warnings (do NOT fail for these alone): style issues, minor naming, dead imports.

Analyze the diff below and return the JSON object.
```

---

## 3. User Prompt Template

```
<system prompt above>

---

Git diff to review:

{DIFF_CONTENT}
```

The diff is appended directly. No extra framing needed.

---

## 4. Expected JSON Output Schema

```json
{
  "approved": true | false,
  "errors": [
    "RULE_NAME: description of the specific violation, referencing the function/line"
  ],
  "warnings": [
    "description of minor issue"
  ],
  "summary": "One sentence describing what the diff changes."
}
```

### Rules for `errors` format
Each error string should be prefixed with the rule name for easy parsing:
```
"UNHANDLED_ERROR: db.Query() return value ignored at line 42"
"DEBUG_LOG: console.log('test') found in saveUser()"
"TYPE_UNSAFE: parameter 'data' typed as 'any' in processPayload()"
"PLACEHOLDER: TODO comment found in handleAuth() — function body is empty"
```

---

## 5. Few-Shot Examples

### ✅ Approved — Clean Change
**Diff input:**
```diff
-async function getUser(id) {
+async function getUser(id: string): Promise<User> {
   try {
     const user = await db.find(id);
     return user;
   } catch (err) {
     logger.error(err);
     throw err;
   }
 }
```
**Expected output:**
```json
{
  "approved": true,
  "errors": [],
  "warnings": [],
  "summary": "Added TypeScript types and error handling to getUser function."
}
```

---

### ❌ Rejected — Unhandled error + debug log
**Diff input:**
```diff
+function saveUser(data) {
+  db.users.insert(data);
+  console.log("saved", data);
+}
```
**Expected output:**
```json
{
  "approved": false,
  "errors": [
    "UNHANDLED_ERROR: db.users.insert() is async but called without await in saveUser()",
    "DEBUG_LOG: console.log() left in saveUser()"
  ],
  "warnings": [],
  "summary": "Added saveUser function that inserts a user into the database."
}
```

---

## 6. Rules Toggle (config-driven prompt injection)

If a rule is disabled in `nano-guard.config.json`, the corresponding rule line is **removed from the prompt** before sending. This keeps the prompt lean and prevents the model from checking things the user doesn't care about.

```go
// internal/evaluator/prompt.go
func BuildSystemPrompt(rules config.Rules) string {
    lines := []string{basePrompt}
    if rules.UnhandledErrors  { lines = append(lines, "1. UNHANDLED_ERROR: ...") }
    if rules.DebugLogs        { lines = append(lines, "2. DEBUG_LOG: ...") }
    if rules.TypeSafety       { lines = append(lines, "3. TYPE_UNSAFE: ...") }
    if rules.PlaceholderStubs { lines = append(lines, "4. PLACEHOLDER: ...") }
    return strings.Join(lines, "\n")
}
```
