-- Поиск элемента в переднем окне процесса через доступность (029 design §3.3, шаги 05, 06, 10).
-- osascript axfind.applescript <bundle id> <find|click|selected|menu> [<надпись>…]
--   find     — кнопка с одной из надписей: прокрутить к ней (AXScrollToVisible), вернуть рамку;
--   click    — то же и нажать;
--   selected — первый выделенный элемент, в имени которого есть одна из надписей (значок в Finder);
--   menu     — то же и открыть его контекстное меню (AXShowMenu);
--   scroll   — правую вертикальную полосу прокрутки окна в конец; вывод — «scroll <x>» или пусто.
-- Вывод: «x y w h<TAB>надпись» в точках экрана (верх-лево), пусто — не нашли.
-- Надписи сравниваются без учёта регистра; «ё» и «е» — разные буквы, вызывающий даёт оба варианта.

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
		repeat with el in els
			set res to my probe(el, act, wanted)
			if res is not "" then return res
		end repeat
	end timeout
	return ""
end run

-- Проверить один элемент; ошибки доступности (элемент исчез, нет атрибута) — «не подошёл»
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
		tell application "System Events"
			if role of el is not "AXButton" then return ""
		end tell
		set nm to my label_of(el)
		if nm is "" or wanted does not contain nm then return ""
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
					click el
				end try
			end tell
		end if
		return res
	on error
		return ""
	end try
end probe

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
