# Schema Docs

- [1. Property `root > log_level`](#log_level)
- [2. Property `root > resolve_paths_with_tanka`](#resolve_paths_with_tanka)
- [3. Property `root > jpath`](#jpath)
  - [3.1. root > jpath > jpath items](#jpath_items)
- [4. Property `root > ext_vars`](#ext_vars)
  - [4.1. Property `root > ext_vars > additionalProperties`](#ext_vars_additionalProperties)
- [5. Property `root > ext_code`](#ext_code)
  - [5.1. Property `root > ext_code > additionalProperties`](#ext_code_additionalProperties)
- [6. Property `root > formatting`](#formatting)
  - [6.1. Property `root > formatting > Indent`](#formatting_Indent)
  - [6.2. Property `root > formatting > MaxBlankLines`](#formatting_MaxBlankLines)
  - [6.3. Property `root > formatting > StringStyle`](#formatting_StringStyle)
  - [6.4. Property `root > formatting > CommentStyle`](#formatting_CommentStyle)
  - [6.5. Property `root > formatting > PrettyFieldNames`](#formatting_PrettyFieldNames)
  - [6.6. Property `root > formatting > PadArrays`](#formatting_PadArrays)
  - [6.7. Property `root > formatting > PadObjects`](#formatting_PadObjects)
  - [6.8. Property `root > formatting > SortImports`](#formatting_SortImports)
  - [6.9. Property `root > formatting > UseImplicitPlus`](#formatting_UseImplicitPlus)
  - [6.10. Property `root > formatting > StripEverything`](#formatting_StripEverything)
  - [6.11. Property `root > formatting > StripComments`](#formatting_StripComments)
  - [6.12. Property `root > formatting > StripAllButComments`](#formatting_StripAllButComments)
- [7. Property `root > diagnostics`](#diagnostics)
  - [7.1. Property `root > diagnostics > enable_eval_diagnostics`](#diagnostics_enable_eval_diagnostics)
  - [7.2. Property `root > diagnostics > enable_lint_diagnostics`](#diagnostics_enable_lint_diagnostics)
- [8. Property `root > inlay`](#inlay)
  - [8.1. Property `root > inlay > enable_debug_ast`](#inlay_enable_debug_ast)
  - [8.2. Property `root > inlay > enable_index_value`](#inlay_enable_index_value)
  - [8.3. Property `root > inlay > enable_function_args`](#inlay_enable_function_args)
  - [8.4. Property `root > inlay > function_args`](#inlay_function_args)
    - [8.4.1. Property `root > inlay > function_args > show_with_same_name`](#inlay_function_args_show_with_same_name)
  - [8.5. Property `root > inlay > max_length`](#inlay_max_length)
- [9. Property `root > enable_semantic_tokens`](#enable_semantic_tokens)
- [10. Property `root > workarounds`](#workarounds)
  - [10.1. Property `root > workarounds > assume_true_condition_on_error`](#workarounds_assume_true_condition_on_error)
- [11. Property `root > completion`](#completion)
  - [11.1. Property `root > completion > enable_snippets`](#completion_enable_snippets)
  - [11.2. Property `root > completion > enable_keywords`](#completion_enable_keywords)
  - [11.3. Property `root > completion > use_type_in_detail`](#completion_use_type_in_detail)
  - [11.4. Property `root > completion > show_docstring`](#completion_show_docstring)

|                           |                       |
| ------------------------- | --------------------- |
| **Type**                  | `object`              |
| **Required**              | No                    |
| **Additional properties** | Not allowed           |
| **Defined in**            | #/$defs/Configuration |

| Property                                                 | Type            | Title/Description                                                                    |
| -------------------------------------------------------- | --------------- | ------------------------------------------------------------------------------------ |
| - [log_level](#log_level )                               | integer         | The log level to use (logrus format)                                                 |
| - [resolve_paths_with_tanka](#resolve_paths_with_tanka ) | boolean         | Use Tanka to resolve paths                                                           |
| - [jpath](#jpath )                                       | array of string | String array with jpaths to add. Defaults to the environment variable "JSONNET_PATH" |
| - [ext_vars](#ext_vars )                                 | object          | String map of ext_vars to use. Key is the name of the var                            |
| - [ext_code](#ext_code )                                 | object          | Same as ext_vars but for extCode                                                     |
| - [formatting](#formatting )                             | object          | Formatting options to use                                                            |
| - [diagnostics](#diagnostics )                           | object          | -                                                                                    |
| - [inlay](#inlay )                                       | object          | -                                                                                    |
| - [enable_semantic_tokens](#enable_semantic_tokens )     | boolean         | Enables semantic tokens                                                              |
| - [workarounds](#workarounds )                           | object          | -                                                                                    |
| - [completion](#completion )                             | object          | -                                                                                    |

## <a name="log_level"></a>1. Property `root > log_level`

|              |           |
| ------------ | --------- |
| **Type**     | `integer` |
| **Required** | No        |

**Description:** The log level to use (logrus format)

## <a name="resolve_paths_with_tanka"></a>2. Property `root > resolve_paths_with_tanka`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

**Description:** Use Tanka to resolve paths

## <a name="jpath"></a>3. Property `root > jpath`

|              |                   |
| ------------ | ----------------- |
| **Type**     | `array of string` |
| **Required** | No                |

**Description:** String array with jpaths to add. Defaults to the environment variable "JSONNET_PATH"

|                      | Array restrictions |
| -------------------- | ------------------ |
| **Min items**        | N/A                |
| **Max items**        | N/A                |
| **Items unicity**    | False              |
| **Additional items** | False              |
| **Tuple validation** | See below          |

| Each item of this array must be | Description |
| ------------------------------- | ----------- |
| [jpath items](#jpath_items)     | -           |

### <a name="jpath_items"></a>3.1. root > jpath > jpath items

|              |          |
| ------------ | -------- |
| **Type**     | `string` |
| **Required** | No       |

## <a name="ext_vars"></a>4. Property `root > ext_vars`

|                           |                                                                                       |
| ------------------------- | ------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                              |
| **Required**              | No                                                                                    |
| **Additional properties** | [Each additional property must conform to the schema](#ext_vars_additionalProperties) |

**Description:** String map of ext_vars to use. Key is the name of the var

| Property                              | Type   | Title/Description |
| ------------------------------------- | ------ | ----------------- |
| - [](#ext_vars_additionalProperties ) | string | -                 |

### <a name="ext_vars_additionalProperties"></a>4.1. Property `root > ext_vars > additionalProperties`

|              |          |
| ------------ | -------- |
| **Type**     | `string` |
| **Required** | No       |

## <a name="ext_code"></a>5. Property `root > ext_code`

|                           |                                                                                       |
| ------------------------- | ------------------------------------------------------------------------------------- |
| **Type**                  | `object`                                                                              |
| **Required**              | No                                                                                    |
| **Additional properties** | [Each additional property must conform to the schema](#ext_code_additionalProperties) |

**Description:** Same as ext_vars but for extCode

| Property                              | Type   | Title/Description |
| ------------------------------------- | ------ | ----------------- |
| - [](#ext_code_additionalProperties ) | string | -                 |

### <a name="ext_code_additionalProperties"></a>5.1. Property `root > ext_code > additionalProperties`

|              |          |
| ------------ | -------- |
| **Type**     | `string` |
| **Required** | No       |

## <a name="formatting"></a>6. Property `root > formatting`

|                           |                 |
| ------------------------- | --------------- |
| **Type**                  | `object`        |
| **Required**              | No              |
| **Additional properties** | Not allowed     |
| **Defined in**            | #/$defs/Options |

**Description:** Formatting options to use

| Property                                                  | Type    | Title/Description |
| --------------------------------------------------------- | ------- | ----------------- |
| - [Indent](#formatting_Indent )                           | integer | -                 |
| - [MaxBlankLines](#formatting_MaxBlankLines )             | integer | -                 |
| - [StringStyle](#formatting_StringStyle )                 | integer | -                 |
| - [CommentStyle](#formatting_CommentStyle )               | integer | -                 |
| - [PrettyFieldNames](#formatting_PrettyFieldNames )       | boolean | -                 |
| - [PadArrays](#formatting_PadArrays )                     | boolean | -                 |
| - [PadObjects](#formatting_PadObjects )                   | boolean | -                 |
| - [SortImports](#formatting_SortImports )                 | boolean | -                 |
| - [UseImplicitPlus](#formatting_UseImplicitPlus )         | boolean | -                 |
| - [StripEverything](#formatting_StripEverything )         | boolean | -                 |
| - [StripComments](#formatting_StripComments )             | boolean | -                 |
| - [StripAllButComments](#formatting_StripAllButComments ) | boolean | -                 |

### <a name="formatting_Indent"></a>6.1. Property `root > formatting > Indent`

|              |           |
| ------------ | --------- |
| **Type**     | `integer` |
| **Required** | No        |

### <a name="formatting_MaxBlankLines"></a>6.2. Property `root > formatting > MaxBlankLines`

|              |           |
| ------------ | --------- |
| **Type**     | `integer` |
| **Required** | No        |

### <a name="formatting_StringStyle"></a>6.3. Property `root > formatting > StringStyle`

|              |           |
| ------------ | --------- |
| **Type**     | `integer` |
| **Required** | No        |

### <a name="formatting_CommentStyle"></a>6.4. Property `root > formatting > CommentStyle`

|              |           |
| ------------ | --------- |
| **Type**     | `integer` |
| **Required** | No        |

### <a name="formatting_PrettyFieldNames"></a>6.5. Property `root > formatting > PrettyFieldNames`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

### <a name="formatting_PadArrays"></a>6.6. Property `root > formatting > PadArrays`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

### <a name="formatting_PadObjects"></a>6.7. Property `root > formatting > PadObjects`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

### <a name="formatting_SortImports"></a>6.8. Property `root > formatting > SortImports`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

### <a name="formatting_UseImplicitPlus"></a>6.9. Property `root > formatting > UseImplicitPlus`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

### <a name="formatting_StripEverything"></a>6.10. Property `root > formatting > StripEverything`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

### <a name="formatting_StripComments"></a>6.11. Property `root > formatting > StripComments`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

### <a name="formatting_StripAllButComments"></a>6.12. Property `root > formatting > StripAllButComments`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

## <a name="diagnostics"></a>7. Property `root > diagnostics`

|                           |                          |
| ------------------------- | ------------------------ |
| **Type**                  | `object`                 |
| **Required**              | No                       |
| **Additional properties** | Not allowed              |
| **Defined in**            | #/$defs/DiagnosticConfig |

| Property                                                           | Type    | Title/Description             |
| ------------------------------------------------------------------ | ------- | ----------------------------- |
| - [enable_eval_diagnostics](#diagnostics_enable_eval_diagnostics ) | boolean | Enable evaluation diagnostics |
| - [enable_lint_diagnostics](#diagnostics_enable_lint_diagnostics ) | boolean | Enable linting diagnostics    |

### <a name="diagnostics_enable_eval_diagnostics"></a>7.1. Property `root > diagnostics > enable_eval_diagnostics`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

**Description:** Enable evaluation diagnostics

### <a name="diagnostics_enable_lint_diagnostics"></a>7.2. Property `root > diagnostics > enable_lint_diagnostics`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

**Description:** Enable linting diagnostics

## <a name="inlay"></a>8. Property `root > inlay`

|                           |                            |
| ------------------------- | -------------------------- |
| **Type**                  | `object`                   |
| **Required**              | No                         |
| **Additional properties** | Not allowed                |
| **Defined in**            | #/$defs/ConfigurationInlay |

| Property                                               | Type    | Title/Description                                  |
| ------------------------------------------------------ | ------- | -------------------------------------------------- |
| - [enable_debug_ast](#inlay_enable_debug_ast )         | boolean | Enables debug ast hints                            |
| - [enable_index_value](#inlay_enable_index_value )     | boolean | Resolves some index values                         |
| - [enable_function_args](#inlay_enable_function_args ) | boolean | Shows the names of unnamed parameters in functions |
| - [function_args](#inlay_function_args )               | object  | -                                                  |
| - [max_length](#inlay_max_length )                     | integer | Max length of inlay hints                          |

### <a name="inlay_enable_debug_ast"></a>8.1. Property `root > inlay > enable_debug_ast`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

**Description:** Enables debug ast hints

### <a name="inlay_enable_index_value"></a>8.2. Property `root > inlay > enable_index_value`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

**Description:** Resolves some index values

### <a name="inlay_enable_function_args"></a>8.3. Property `root > inlay > enable_function_args`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

**Description:** Shows the names of unnamed parameters in functions

### <a name="inlay_function_args"></a>8.4. Property `root > inlay > function_args`

|                           |                           |
| ------------------------- | ------------------------- |
| **Type**                  | `object`                  |
| **Required**              | No                        |
| **Additional properties** | Not allowed               |
| **Defined in**            | #/$defs/InlayFunctionArgs |

| Property                                                           | Type    | Title/Description                                              |
| ------------------------------------------------------------------ | ------- | -------------------------------------------------------------- |
| - [show_with_same_name](#inlay_function_args_show_with_same_name ) | boolean | Show inlay hints for parameters even if the names are the same |

#### <a name="inlay_function_args_show_with_same_name"></a>8.4.1. Property `root > inlay > function_args > show_with_same_name`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

**Description:** Show inlay hints for parameters even if the names are the same

### <a name="inlay_max_length"></a>8.5. Property `root > inlay > max_length`

|              |           |
| ------------ | --------- |
| **Type**     | `integer` |
| **Required** | No        |

**Description:** Max length of inlay hints

## <a name="enable_semantic_tokens"></a>9. Property `root > enable_semantic_tokens`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

**Description:** Enables semantic tokens

## <a name="workarounds"></a>10. Property `root > workarounds`

|                           |                          |
| ------------------------- | ------------------------ |
| **Type**                  | `object`                 |
| **Required**              | No                       |
| **Additional properties** | Not allowed              |
| **Defined in**            | #/$defs/WorkaroundConfig |

| Property                                                                         | Type    | Title/Description                                                                                              |
| -------------------------------------------------------------------------------- | ------- | -------------------------------------------------------------------------------------------------------------- |
| - [assume_true_condition_on_error](#workarounds_assume_true_condition_on_error ) | boolean | Assumes all conditions to be true if they run into an error (since currently not all conditions are supported) |

### <a name="workarounds_assume_true_condition_on_error"></a>10.1. Property `root > workarounds > assume_true_condition_on_error`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

**Description:** Assumes all conditions to be true if they run into an error (since currently not all conditions are supported)

## <a name="completion"></a>11. Property `root > completion`

|                           |                          |
| ------------------------- | ------------------------ |
| **Type**                  | `object`                 |
| **Required**              | No                       |
| **Additional properties** | Not allowed              |
| **Defined in**            | #/$defs/CompletionConfig |

| Property                                                | Type    | Title/Description                                                                                                                                              |
| ------------------------------------------------------- | ------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| - [enable_snippets](#completion_enable_snippets )       | boolean | Enable support for snippets. These are still broken in a bunch of cases. E.g. this allows to complete \`array.length\` which resolves to \`std.length(array)\` |
| - [enable_keywords](#completion_enable_keywords )       | boolean | Enable support for completing keywords. e.g. self, super, local etc.                                                                                           |
| - [use_type_in_detail](#completion_use_type_in_detail ) | boolean | Puts the target type in the \`detail\` field. Try this if you don't see any type info                                                                          |
| - [show_docstring](#completion_show_docstring )         | boolean | Show documentation in completion (fields beginning with #)                                                                                                     |

### <a name="completion_enable_snippets"></a>11.1. Property `root > completion > enable_snippets`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

**Description:** Enable support for snippets. These are still broken in a bunch of cases. E.g. this allows to complete `array.length` which resolves to `std.length(array)`

### <a name="completion_enable_keywords"></a>11.2. Property `root > completion > enable_keywords`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

**Description:** Enable support for completing keywords. e.g. self, super, local etc.

### <a name="completion_use_type_in_detail"></a>11.3. Property `root > completion > use_type_in_detail`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

**Description:** Puts the target type in the `detail` field. Try this if you don't see any type info

### <a name="completion_show_docstring"></a>11.4. Property `root > completion > show_docstring`

|              |           |
| ------------ | --------- |
| **Type**     | `boolean` |
| **Required** | No        |

**Description:** Show documentation in completion (fields beginning with #)

----------------------------------------------------------------------------------------------------------------------------
Generated using [json-schema-for-humans](https://github.com/coveooss/json-schema-for-humans) on 2025-06-19 at 18:47:41 +0200
