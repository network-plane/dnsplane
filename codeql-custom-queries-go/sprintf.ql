import go

/**
 * Detects unsafe use of fmt.Sprintf where untrusted input may be used.
 */

// Define a source of untrusted input (e.g., reading from a network or an API call)
class UntrustedSource extends Expr {
  UntrustedSource() {
    this = any(CallExpr call |
      call.getTarget().hasQualifiedName("net/http", "Request") or  // HTTP request
      call.getTarget().hasQualifiedName("os", "Args") or           // Command-line arguments
      call.getTarget().hasQualifiedName("gin", "Context.Param") or // Gin framework parameters
      call.getTarget().hasQualifiedName("io/ioutil", "ReadAll")    // Reading data from untrusted files
    )
  }
}


from CallExpr sprintfCall, Expr input
where
    sprintfCall.getTarget().hasQualifiedName("fmt", "Sprintf") and
    input = sprintfCall.getArgument(_) and
    input instanceof UntrustedSource
select sprintfCall, input, "Unsafe use of fmt.Sprintf with untrusted input."
