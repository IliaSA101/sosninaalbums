-- Поиск элемента в переднем окне процесса через доступность (029 design §3.3, шаги 05, 06, 10).
-- osascript axfind.applescript <bundle id> <find|click|selected|menu> [<надпись>…]
--   find     — элемент с одной из надписей (любая роль; нажимаемый — сам или предок с AXPress): прокрутить
--              к нему (AXScrollToVisible), вернуть рамку;
--   click    — то же и нажать (AXPress, запасной путь — щелчок в центр рамки);
--   dump     — дерево доступности окна: роль, AXTitle, AXDescription, AXValue, AXIdentifier, рамка;
--   selected — первый выделенный элемент, в имени которого есть одна из надписей (значок в Finder);
--   menu     — то же и открыть его контекстное меню (AXShowMenu);
--   scroll   — правую вертикальную полосу прокрутки окна в конец; вывод — «scroll <x>» или пусто.
-- Вывод: «x y w h<TAB>надпись» в точках экрана (верх-лево), пусто — не нашли.
-- Надписи сравниваются по «содержит», без учёта регистра, «ё»=«е», неразрывный пробел = пробел.

on run argv
	set bid to item 1 of argv
	set act to item 2 of argv
	set wanted to {}
	if (count of argv) > 2 then set wanted to items 3 thru -1 of argv
	with timeout of 100 seconds
		tell application "System Events"
			set procs to (every process whose bundle identifier is bid)
			if procs is {} then return ""
			set p to item 1 of procs
			if (count of windows of p) is 0 then return ""
			set els to entire contents of window 1 of p
		end tell
		if act is "scroll" then return my scroll_down(els)
		if act is "dump" then return my dump_tree(els)
		if act is "find" or act is "click" then return my find_pressable(els, act, wanted)
		repeat with el in els
			set res to my probe(el, act, wanted)
			if res is not "" then return res
		end repeat
	end timeout
	return ""
end run

-- Проверить один элемент в режимах selected/menu; ошибки доступности (элемент исчез) — «не подошёл»
on probe(el, act, wanted)
	try
		if act is "selected" or act is "menu" then
			tell application "System Events"
				if (value of attribute "AXSelected" of el) is not true then return ""
			end tell
			-- выделена и строка «Загрузки» в боковой панели: нужен элемент с именем файла
			set nm to my label_of(el)
			if wanted is not {} and not my has_any(nm, wanted) then return ""
			set res to my report(el, nm)
			-- AXShowMenu может не вернуться, пока меню открыто: вызывающий запускает «menu» в фоне
			if act is "menu" then
				tell application "System Events" to perform action "AXShowMenu" of el
			end if
			return res
		end if
		return ""
	on error
		return ""
	end try
end probe

-- Поиск надписи в любом элементе окна (прогон 2: кнопку «Все равно открыть» по роли AXButton и точному
-- имени не нашли — SwiftUI кладёт надпись в AXTitle/AXDescription/AXValue или во вложенный AXStaticText).
-- Нажимаемая цель — сам элемент или его предок (до 4 уровней) с действием AXPress; не нашлась —
-- рамка первого совпавшего (для click — щелчок в её центр).
on find_pressable(els, act, wanted)
	set nwanted to {}
	repeat with w in wanted
		set end of nwanted to my norm(w as text)
	end repeat
	set fallback to missing value
	repeat with el in els
		try
			set hit to my label_hit(el, nwanted)
			if hit is not "" then
				set tgt to my pressable_of(el)
				if tgt is missing value then
					if fallback is missing value then set fallback to {el, hit}
				else
					return my act_on(tgt, hit, act)
				end if
			end if
		end try
	end repeat
	if fallback is not missing value then return my act_on(item 1 of fallback, item 2 of fallback, act)
	return ""
end find_pressable

on act_on(el, nm, act)
	tell application "System Events"
		try
			perform action "AXScrollToVisible" of el
			delay 0.5
		end try
	end tell
	set res to my report(el, nm)
	if act is "click" then
		tell application "System Events"
			try
				perform action "AXPress" of el
			on error
				try
					set pos to position of el
					set sz to size of el
					click at {(item 1 of pos) + (item 1 of sz) / 2, (item 2 of pos) + (item 2 of sz) / 2}
				end try
			end try
		end tell
	end if
	return res
end act_on

-- Подпись элемента, совпавшая с одной из надписей (пусто — нет). Короткие надписи («OK») — только целиком,
-- длинные — «содержит», но без абзацев: подпись не длиннее надписи больше чем на 20 знаков.
on label_hit(el, nwanted)
	set labs to my labels_of(el)
	repeat with raw in labs
		set t to my norm(raw as text)
		if t is not "" then
			repeat with w in nwanted
				set wl to length of (w as text)
				if wl < 6 then
					if t is (w as text) then return raw as text
				else if (length of t) <= wl + 20 and t contains (w as text) then
					return raw as text
				end if
			end repeat
		end if
	end repeat
	return ""
end label_hit

on pressable_of(el)
	set cur to el
	repeat 5 times
		if my can_press(cur) then return cur
		try
			tell application "System Events" to set cur to value of attribute "AXParent" of cur
		on error
			return missing value
		end try
	end repeat
	return missing value
end pressable_of

on can_press(el)
	try
		tell application "System Events" to set acts to name of actions of el
		return acts contains "AXPress"
	on error
		return false
	end try
end can_press

-- Подписи элемента: name, description, AXTitle, AXValue (только текстовые, не пустые)
on labels_of(el)
	set out to {}
	tell application "System Events"
		repeat with getter in {"name", "description", "AXTitle", "AXValue"}
			try
				if getter as text is "name" then
					set v to name of el
				else if getter as text is "description" then
					set v to description of el
				else
					set v to value of attribute (getter as text) of el
				end if
				if v is not missing value then
					set v to v as text
					if v is not "" then set end of out to v
				end if
			end try
		end repeat
	end tell
	return out
end labels_of

-- «ё»→«е», неразрывные пробелы → обычные; регистр AppleScript в сравнениях не учитывает
on norm(t)
	set t to my str_replace(t, "ё", "е")
	set t to my str_replace(t, "Ё", "е")
	set t to my str_replace(t, character id 160, " ")
	set t to my str_replace(t, character id 8239, " ")
	return t
end norm

on str_replace(t, a, b)
	set od to AppleScript's text item delimiters
	set AppleScript's text item delimiters to a
	set parts to text items of t
	set AppleScript's text item delimiters to b
	set t to parts as text
	set AppleScript's text item delimiters to od
	return t
end str_replace

-- Дерево окна для logs/ax-security.txt: «№ TAB роль TAB AXTitle TAB AXDescription TAB AXValue TAB
-- AXIdentifier TAB x y w h». entire contents уже отдаёт элементы любой вложенности (глубже 15 тоже).
on dump_tree(els)
	set out to ""
	set n to 0
	repeat with el in els
		set n to n + 1
		if n > 3000 then exit repeat
		set cols to {n as text}
		tell application "System Events"
			repeat with a in {"AXRole", "AXTitle", "AXDescription", "AXValue", "AXIdentifier"}
				try
					set v to value of attribute (a as text) of el
					if v is missing value then set v to ""
					set end of cols to my flat(v as text)
				on error
					set end of cols to ""
				end try
			end repeat
			try
				set pos to position of el
				set sz to size of el
				set end of cols to ((round (item 1 of pos)) as text) & " " & ((round (item 2 of pos)) as text) & " " & ((round (item 1 of sz)) as text) & " " & ((round (item 2 of sz)) as text)
			on error
				set end of cols to ""
			end try
		end tell
		set od to AppleScript's text item delimiters
		set AppleScript's text item delimiters to tab
		set out to out & (cols as text) & linefeed
		set AppleScript's text item delimiters to od
	end repeat
	return out
end dump_tree

on flat(t)
	set t to my str_replace(t, linefeed, " ")
	set t to my str_replace(t, return, " ")
	return my str_replace(t, tab, " ")
end flat

-- SwiftUI строит нижние строки «Настроек» только при прокрутке: до неё кнопки «Открыть всё равно»
-- в дереве доступности нет (прогон 1). Правая полоса — панель раздела, левая — боковое меню.
-- Сдвиг дважды: после первого подгружаются строки и низ уезжает.
on scroll_down(els)
	set best to missing value
	set bestX to -100000
	repeat with el in els
		try
			tell application "System Events"
				if role of el is "AXScrollBar" then
					if (value of attribute "AXOrientation" of el) is "AXVerticalOrientation" then
						set sx to item 1 of (get position of el)
						if sx > bestX then
							set bestX to sx
							set best to el
						end if
					end if
				end if
			end tell
		end try
	end repeat
	if best is missing value then return ""
	repeat 2 times
		try
			tell application "System Events" to set value of attribute "AXValue" of best to 1.0
		on error
			try
				tell application "System Events" to set value of best to 1.0
			end try
		end try
		delay 1
	end repeat
	return "scroll " & ((round bestX) as text)
end scroll_down

-- Надпись кнопки: SwiftUI кладёт её то в name, то в description, то в AXTitle
on label_of(el)
	tell application "System Events"
		repeat with getter in {"name", "description", "title"}
			try
				if getter as text is "name" then
					set v to name of el
				else if getter as text is "description" then
					set v to description of el
				else
					set v to value of attribute "AXTitle" of el
				end if
				if v is not missing value and (v as text) is not "" then return v as text
			end try
		end repeat
	end tell
	return ""
end label_of

on has_any(nm, parts)
	repeat with p in parts
		if nm contains (p as text) then return true
	end repeat
	return false
end has_any

on report(el, nm)
	tell application "System Events"
		set pos to position of el
		set sz to size of el
	end tell
	set t to (round (item 1 of pos)) as text
	set t to t & " " & ((round (item 2 of pos)) as text)
	set t to t & " " & ((round (item 1 of sz)) as text)
	set t to t & " " & ((round (item 2 of sz)) as text)
	return t & tab & nm
end report
