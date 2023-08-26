# Check all Conditions

Tiny tool to check all conditions of all resources in your Kubernetes cluster.

Takes roughly 15 seconds, even in small cluster.

Please provide feedback, PRs are welcome.

# Background and Goal

I develop Kubernetes controllers in Go. I develop software since ages, 
but Kubernetes and Go are still a bit new to me.

I as a developer of a controllers want an overview. I want to see the difference between
the desired state and the observed state.


If a controller discover that the observed state does not match the desired state,
it could ...

... could write logs. But logs are just dust in the wind. After the next reconciliation,
the log message will be outdated.

... could emit events. Same here: After the next reconciliation the event could be outdated.

... could write to status.conditions. That's what we currently do.

But how to monitor many conditions of many resources?

I found no tool which monitors all conditions of all resource objects. So I wrote this tiny tool.

# Executing

```
go run github.com/guettli/check-conditions@latest
```

# Terminology

Since I found not good umbrella term for CRDs and core resource types, I use the term CRD.

Related: [Kubernetes API Terminology](https://kubernetes.io/docs/reference/using-api/api-concepts/#standard-api-terminology)

# OK vs Warning?

Which conditions should create output and which conditions are ok and can get ignored?

Up to now the code contains some simple lists. 

Examples:

* *Ready=True will be ignored
* *Healthy=True will be ignored
* *Pressure=False will be ignored.



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


