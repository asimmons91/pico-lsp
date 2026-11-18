-- simple cart
function _init()
  player={x=64,y=64,dx=0,dy=0}
  score=0
end

function _update()
  player.x+=player.dx
  player.y+=player.dy
  if btn(0) then player.dx-=1 end
  if btn(1) then player.dx+=1 end
  if btn(2) then player.dy-=1 end
  if btn(3) then player.dy+=1 end
  player.dx*=0.9
  player.dy*=0.9
end

function _draw()
  cls(0)
  circfill(player.x,player.y,4,12)
  print("score: "..score,2,2,7)
end
