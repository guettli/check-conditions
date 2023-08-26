# Check all Conditions

Tiny tool to check all conditions of all resources in your Kubernetes cluster.

Takes roughly 15 seconds, even in small cluster.

Please provide feedback, PRs are welcome.

# Executing

```
go run github.com/guettli/check-conditions@latest
```

# Terminology

Since I found not good umbrella term for CRDs and core resource types, I use the term CRD.

Related: [Kubernetes API Terminology](https://kubernetes.io/docs/reference/using-api/api-concepts/#standard-api-terminology)

# Ideas

Continously watch all resources for changes, monitor all changes.

Report broken ownerRefs.

HTML GUI via localhost.

Negative conditions are ok for a defined time period. 
Example: It is ok if a Pod needs 20 seconds to start.
But it is not ok if it takes 5 minutes.

To make warnings appear sooner after starting the programm 
(it takes 20 secs even for small clusters), we could
use some kind of priority. CRDs which had warnings in the past, should
be checked sooner. This state could be stored in $XDG_CACHE_HOME.

Eval more than conditions. Everything should be possible.
How to make ignoring or adding some warnings super flexible?
The most simple way would be to use Go code.


