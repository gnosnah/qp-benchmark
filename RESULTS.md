# Benchmark results

Full median tables for `github.com/gnosnah/qp@v1.0.0`. N=10,000; 1 CPU; 3×100ms
samples; ns/op unless noted. Headline summary: [README.md](README.md).

| Workload / key shape | qp | map | B-tree | radix |
|---|---:|---:|---:|---:|
| Build / random16 | 3,587,424 | 1,002,106 | 5,188,990 | 10,191,144 |
| Build / prefix64 | 6,580,678 | 1,254,393 | 6,089,500 | 11,938,859 |
| Build / domain | 4,570,833 | 1,285,727 | 5,211,425 | 13,701,504 |
| Get hit / random16 | 75.82 | 26.70 | 378.0 | 260.5 |
| Get hit / prefix64 | 98.65 | 50.90 | 500.1 | 263.7 |
| Get hit / domain | 91.84 | 50.29 | 411.3 | 304.8 |
| Get miss / random16 | 62.20 | 22.70 | 512.9 | 248.7 |
| Get miss / prefix64 | 66.26 | 36.79 | 541.2 | 363.5 |
| Get miss / domain | 69.05 | 29.85 | 439.6 | 299.3 |
| Snapshot update 16 / random16 | 18,773 | 302,137 | 27,384 | 40,988 |
| Snapshot update 16 / prefix64 | 15,412 | 257,635 | 23,139 | 64,476 |
| Snapshot update 16 / domain | 14,989 | 222,622 | 23,044 | 53,865 |

Other ops (random16 only):

| Workload | qp | map | B-tree | radix |
|---|---:|---:|---:|---:|
| Update one existing key | 118.9 | 80.96 | 397.4 | 8,492 |
| Delete all 10,000 keys | 1,258,193 | 628,910 | 5,020,195 | 76,905,272 |
| Ordered scan of 10,000 keys | 498,144 | 2,403,339 | 56,491 | 125,593 |

Raw samples: [random16](benchmark-results.txt),
[prefix64](benchmark-prefix64.txt), [domain](benchmark-domain.txt).
