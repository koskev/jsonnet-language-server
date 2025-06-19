# Jsonnet Language Server

A **[Language Server Protocol (LSP)](https://langserver.org)** server for [Jsonnet](https://jsonnet.org).

Master is (probably) always somehow broken or not tested on a complex codebase. Tags should be stable though

## Features of this fork

 * Autocomplete function parameter names
 * Autocomplete function parameter with their default values
 * Find references
 * Rename
 * Very basic signature help
 * Inlay hints
   * unnamed function parameters
   * Index (e.g. extcode variables)
 * Bugfixes
    * Infinite loops
    * Completion with named args and objects without a space
    * Binary Object completion
  * Complete rework of the autocomplete code
    * Proper extcode support
    * Complete extVar/Code variables and all valid values e.g. std.extVar("**PARAMS**")
    * Complete for objects
    * Supports completion after newlines and whitespaces e.g. myobj.\n  myVal
    * Completes returns of functions
      * e.g. myFunc({val: 5}).argVal.val
      * Builder pattern
    * Complete imports from all jpaths
    * Complete import function calls (import 'a.libsonnet')("myArg").val
    * Support super completion
    * Complete keywords (super, self, local)
    * Complete conditionals
      * Currently not all conditions are supported
    * Complete array access
    * Complete unused argument names: myFunc(1, arg3=3, ar**g2=**),
  * Basic automatic ast fix
  * Basic semantic token support
    * Only the basic stuff. It is assumed you are also using something like tree sitter

### TODO
 * Reimplement
    * Go to definition
    * Inlay hints
 * Refactor/Cleanup new features
 * General cleanup after understanding more lsp stuff
 * Code actions?
 * Flow typing?

## Features

### LSP Config parameter

[Documentation](schema_doc.md)
Full default Config:
```json
{
  "log_level": "panic",
  "resolve_paths_with_tanka": false,
  "jpath": [],
  "ext_vars": null,
  "ext_code": null,
  "formatting": {
    "Indent": 2,
    "MaxBlankLines": 2,
    "StringStyle": 1,
    "CommentStyle": 1,
    "PrettyFieldNames": true,
    "PadArrays": false,
    "PadObjects": true,
    "SortImports": true,
    "UseImplicitPlus": true,
    "StripEverything": false,
    "StripComments": false,
    "StripAllButComments": false
  },
  "diagnostics": {
    "enable_eval_diagnostics": false,
    "enable_lint_diagnostics": false
  },
  "inlay": {
    "enable_debug_ast": false,
    "enable_index_value": false,
    "enable_function_args": false,
    "function_args": {
      "show_with_same_name": false
    },
    "max_length": 120
  },
  "enable_semantic_tokens": false,
  "workarounds": {
    "assume_true_condition_on_error": false
  },
  "completion": {
    "enable_snippets": false,
    "enable_keywords": true,
    "use_type_in_detail": false,
    "show_docstring": false
  }
}
```


### New features

https://github.com/user-attachments/assets/0530595f-ef16-4716-a6cb-9f5c67b37b65

https://github.com/user-attachments/assets/66c30d80-8c2e-42e9-8878-2aeda30ae3ba

https://github.com/user-attachments/assets/712d4850-4a34-486c-9443-b5ca63af2653



### Jump to definition




https://user-images.githubusercontent.com/29210090/145594957-efc01d97-d4c1-4fad-85cb-f5cb4a5f0e97.mp4

https://user-images.githubusercontent.com/29210090/145594976-670fff41-55e9-4ff9-b104-b5ac1cf77b42.mp4

https://user-images.githubusercontent.com/29210090/154743159-81adf3b3-e929-4731-8b23-718085d222c5.mp4

### Error/Warning Diagnostics

https://user-images.githubusercontent.com/29210090/145595007-59dd4276-e8c2-451e-a1d9-bfc7fd83923f.mp4

### Linting Diagnostics

https://user-images.githubusercontent.com/29210090/145595044-ca3f09cf-5806-4586-8aa8-720b6927bc6d.mp4

### Standard Library Hover and Autocomplete

https://user-images.githubusercontent.com/29210090/145595059-e34c6d25-eff3-41df-ae4a-d3713ee35360.mp4

### Formatting

## Installation

Download the latest release binary from GitHub: https://github.com/grafana/jsonnet-language-server/releases

## Contributing

Contributions are more than welcome and I will try my best to be prompt
with reviews.

### Commits

Individual commits should be meaningful and have useful commit messages.
For tips on writing commit messages, refer to [How to write a commit
message](https://chris.beams.io/posts/git-commit/). Contributions will
be rebased before merge to ensure a fast-forward merge.

### [Developer Certificate of Origin (DCO)](https://github.com/probot/dco#how-it-works)

Contributors must sign the DCO for their contributions to be accepted.

### Code style

Go code should be formatted with `gofmt` and linted with
[golangci-lint](https://golangci-lint.run/).

## Editor integration

* Emacs: Refer to [editor/emacs](editor/emacs)
* Vim: Refer to [editor/vim](editor/vim)
* VSCod(e|ium): Use the [Jsonnet Language Server extension](https://marketplace.visualstudio.com/items?itemName=Grafana.vscode-jsonnet) ([source code](https://github.com/grafana/vscode-jsonnet))
* Jetbrains: Use the [Jsonnet Language Server plugin](https://plugins.jetbrains.com/plugin/18752-jsonnet-language-server) ([source code](https://github.com/zzehring/intellij-jsonnet))
