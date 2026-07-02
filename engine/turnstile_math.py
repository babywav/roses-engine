import math
import random
import time
from typing import List, Tuple

class HumanMousePhysics:
    """
    Motor físico para geração de movimentos de mouse humanos usando
    Curvas de Bézier Cúbicas, Lei de Fitts (timing) e Injeção de Ruído.
    """

    @staticmethod
    def _cubic_bezier(t: float, p0: Tuple[int, int], p1: Tuple[int, int], p2: Tuple[int, int], p3: Tuple[int, int]) -> Tuple[int, int]:
        """Calcula um ponto numa curva de Bézier cúbica para o tempo t (0.0 a 1.0)."""
        u = 1 - t
        tt = t * t
        uu = u * u
        uuu = uu * u
        ttt = tt * t

        x = uuu * p0[0] + 3 * uu * t * p1[0] + 3 * u * tt * p2[0] + ttt * p3[0]
        y = uuu * p0[1] + 3 * uu * t * p1[1] + 3 * u * tt * p2[1] + ttt * p3[1]
        
        return int(round(x)), int(round(y))

    @staticmethod
    def _ease_out_quad(t: float) -> float:
        """Função de easing para desaceleração orgânica ao chegar no alvo."""
        return t * (2 - t)
        
    @staticmethod
    def _fitts_law_duration(distance: float, target_width: float = 30.0) -> float:
        """
        Calcula o tempo (em segundos) que um humano leva para mover o mouse,
        usando uma aproximação empírica da Lei de Fitts.
        a = tempo de reação (0.2s), b = tempo index (0.15s)
        """
        if distance < 10:
            return random.uniform(0.1, 0.3)
            
        a = random.uniform(0.15, 0.25)
        b = random.uniform(0.10, 0.20)
        
        index_of_difficulty = math.log2(2 * distance / target_width)
        
        if index_of_difficulty < 1:
            index_of_difficulty = 1.0
            
        duration = a + b * index_of_difficulty
        return min(max(duration, 0.3), 1.5)

    @classmethod
    def generate_trajectory(
        cls, 
        start: Tuple[int, int], 
        end: Tuple[int, int], 
        num_points: int = 50
    ) -> List[Tuple[int, int, float]]:
        """
        Gera uma lista de passos (x, y, delay_after_sec).
        - start: Posição inicial (x, y)
        - end: Posição final (alvo) (x, y)
        """
        dx = end[0] - start[0]
        dy = end[1] - start[1]
        distance = math.hypot(dx, dy)
        
        deviation_scale = distance * random.uniform(0.1, 0.4)
        
        angle = math.atan2(dy, dx)
        perp_angle = angle + (math.pi / 2) * random.choice([1, -1])
        
        p1 = (
            start[0] + int(math.cos(angle) * (distance * 0.3) + math.cos(perp_angle) * deviation_scale),
            start[1] + int(math.sin(angle) * (distance * 0.3) + math.sin(perp_angle) * deviation_scale)
        )
        p2 = (
            start[0] + int(math.cos(angle) * (distance * 0.7) - math.cos(perp_angle) * deviation_scale * 0.5),
            start[1] + int(math.sin(angle) * (distance * 0.7) - math.sin(perp_angle) * deviation_scale * 0.5)
        )

        total_duration = cls._fitts_law_duration(distance)
        steps = []
        
        jitter_x, jitter_y = 0, 0
        
        for i in range(num_points):
            t = (i + 1) / num_points
            eased_t = cls._ease_out_quad(t)
            
            base_x, base_y = cls._cubic_bezier(eased_t, start, p1, p2, end)
            
            noise_factor = 1.0 - eased_t
            jitter_x = jitter_x * 0.5 + random.uniform(-3, 3) * noise_factor
            jitter_y = jitter_y * 0.5 + random.uniform(-3, 3) * noise_factor
            
            final_x = base_x + int(round(jitter_x))
            final_y = base_y + int(round(jitter_y))
            
            if i == num_points - 1:
                final_x, final_y = end
                
            delay = total_duration / num_points
            delay *= random.uniform(0.8, 1.2)
            
            steps.append((final_x, final_y, delay))
            
        return steps
