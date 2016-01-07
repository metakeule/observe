# Plan für Observer

wir brauchen folgende asynchrone Zyklen

## fsevent loop:

hier ist es wichtig, dass alle ereignisse schnell entgegengenommen werden und nicht durch
blockierende syscalls ausgebremst werden, daher sollten actionen, wir schreiben in channels
oder hinzufügen neuer verzeichnisse zur watchlist, in neuen goroutinen ablaufen

dann brauchen wir eine channelq, die ausreichend groß ist, um ereignisse entgegen zu nehmen


# dann brauchen wir eine konsolidierungsloop:

die dafür sorgt, dass nur der letzte aufruf einer datei oder überhaupt
der letzte aufruf berücksichtigung findet (über ein gelocktes map)

# dann brauchen wir eine ausführungsloop:

 die sich immer eine datei vom map holt und löscht und ausführt.

optional kann auch ein zeitwert in millisekunden angegeben werden, der auf jeden fall zwischen zwei ausführungen eines
befehls liegen soll

