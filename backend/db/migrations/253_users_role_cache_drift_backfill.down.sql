-- Bewusst leer, aus demselben Grund wie bei 249: die Up-Migration ändert Daten,
-- kein Schema. Ein Rückbau müsste Zeilen, die sie herabgestuft hat, wieder auf
-- 'admin'/'editor' setzen — unterscheiden könnte er sie aber nicht von Zeilen,
-- die schon vorher so standen. Er würde also genau die zu hohen Cache-Werte
-- wiederherstellen, die die Migration entfernt hat.
--
-- Der Rückbau des Codes braucht ihn auch nicht: nach ESK-13 autorisiert users.role
-- nichts mehr, ein herabgestufter Cache-Wert nimmt niemandem etwas weg.
SELECT 1;
